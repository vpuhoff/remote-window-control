using System.Buffers;
using System.Runtime.InteropServices;
using Vortice.Direct3D;
using Vortice.Direct3D11;
using WinRT;
using Windows.Graphics.Capture;
using Windows.Graphics.DirectX;
using Windows.Graphics.DirectX.Direct3D11;

namespace WindowCapture.Native;

public sealed class WgcCaptureService : IDisposable
{
    private static readonly Guid GraphicsCaptureItemIid = new("79C3F95B-31F7-4EC2-A464-632EF5D30760");
    private static readonly Guid GraphicsCaptureItemInteropIid = new("3628E81B-3CAC-4C60-B7F4-23CE0E0C3356");

    private ID3D11Device? _d3dDevice;
    private IDirect3DDevice? _winrtDevice;
    private GraphicsCaptureItem? _captureItem;
    private Direct3D11CaptureFramePool? _framePool;
    private GraphicsCaptureSession? _session;
    private ID3D11Texture2D? _stagingTexture;
    private readonly object _frameLock = new();
    private readonly AutoResetEvent _frameEvent = new(false);
    private byte[]? _frameBuffer;
    private int _frameWidth;
    private int _frameHeight;
    private int _frameStride;
    private int _frameBytesWritten;
    private long _frameId;
    private Exception? _fatalError;
    private bool _disposed;

    public static bool IsSupported() => GraphicsCaptureSession.IsSupported();

    public void StartCapture(IntPtr hwnd)
    {
        ThrowIfDisposed();
        WinRtInitialization.EnsureInitialized();
        InitializeDevices();
        _captureItem = CreateItemForWindow(hwnd);
        _framePool = Direct3D11CaptureFramePool.CreateFreeThreaded(
            _winrtDevice!,
            DirectXPixelFormat.B8G8R8A8UIntNormalized,
            2,
            _captureItem.Size);

        _framePool.FrameArrived += OnFrameArrived;
        _session = _framePool.CreateCaptureSession(_captureItem);
        _session.StartCapture();
    }

    public long WaitForFrame(long lastSeenFrameId, int timeoutMs)
    {
        ThrowIfDisposed();
        ThrowIfFaulted();

        lock (_frameLock)
        {
            if (_frameId > lastSeenFrameId)
            {
                return _frameId;
            }
        }

        var signaled = _frameEvent.WaitOne(timeoutMs < 0 ? Timeout.Infinite : timeoutMs);
        ThrowIfFaulted();
        if (!signaled)
        {
            return 0;
        }

        lock (_frameLock)
        {
            return _frameId > lastSeenFrameId ? _frameId : 0;
        }
    }

    public FrameCopyResult CopyLatestFrame(IntPtr destination, int destinationLength)
    {
        ThrowIfDisposed();
        ThrowIfFaulted();

        lock (_frameLock)
        {
            if (_frameId == 0 || _frameBuffer is null)
            {
                return FrameCopyResult.NoFrame;
            }

            if (destination == IntPtr.Zero || destinationLength < _frameBytesWritten)
            {
                return new FrameCopyResult(
                    CopyFrameStatus.BufferTooSmall,
                    _frameWidth,
                    _frameHeight,
                    _frameStride,
                    _frameBytesWritten,
                    _frameId);
            }

            Marshal.Copy(_frameBuffer, 0, destination, _frameBytesWritten);
            return new FrameCopyResult(
                CopyFrameStatus.Success,
                _frameWidth,
                _frameHeight,
                _frameStride,
                _frameBytesWritten,
                _frameId);
        }
    }

    public void Dispose()
    {
        if (_disposed)
        {
            return;
        }

        _disposed = true;

        if (_framePool is not null)
        {
            _framePool.FrameArrived -= OnFrameArrived;
        }

        _session?.Dispose();
        _session = null;

        _framePool?.Dispose();
        _framePool = null;

        _stagingTexture?.Dispose();
        _stagingTexture = null;

        _winrtDevice?.Dispose();
        _d3dDevice?.Dispose();

        lock (_frameLock)
        {
            if (_frameBuffer is not null)
            {
                ArrayPool<byte>.Shared.Return(_frameBuffer);
                _frameBuffer = null;
            }
        }

        _frameEvent.Dispose();
    }

    private void InitializeDevices()
    {
        if (_d3dDevice is not null && _winrtDevice is not null)
        {
            return;
        }

        var hr = D3D11CreateDeviceNative(
            IntPtr.Zero,
            DriverType.Hardware,
            IntPtr.Zero,
            (uint)DeviceCreationFlags.BgraSupport,
            null,
            0,
            D3D11SdkVersion,
            out var devicePointer,
            out _,
            out var contextPointer);
        Marshal.ThrowExceptionForHR(hr);

        try
        {
            _d3dDevice = new ID3D11Device(devicePointer);
            _winrtDevice = CreateWinRtDevice(_d3dDevice);
        }
        finally
        {
            if (contextPointer != IntPtr.Zero)
            {
                Marshal.Release(contextPointer);
            }
        }
    }

    private void OnFrameArrived(Direct3D11CaptureFramePool sender, object args)
    {
        try
        {
            using var frame = sender.TryGetNextFrame();
            if (frame is null)
            {
                return;
            }

            if (frame.ContentSize.Width <= 0 || frame.ContentSize.Height <= 0)
            {
                return;
            }

            if (_captureItem is not null &&
                (frame.ContentSize.Width != _captureItem.Size.Width || frame.ContentSize.Height != _captureItem.Size.Height))
            {
                sender.Recreate(
                    _winrtDevice!,
                    DirectXPixelFormat.B8G8R8A8UIntNormalized,
                    2,
                    frame.ContentSize);
            }

            var surfaceInterop = frame.Surface.As<IDirect3DDxgiInterfaceAccess>();
            var resourceGuid = typeof(ID3D11Texture2D).GUID;
            var resourcePointer = surfaceInterop.GetInterface(ref resourceGuid);
            using var gpuTexture = new ID3D11Texture2D(resourcePointer);

            EnsureStagingTexture(gpuTexture.Description);
            _d3dDevice!.ImmediateContext.CopyResource(_stagingTexture!, gpuTexture);

            _d3dDevice.ImmediateContext.Map(_stagingTexture!, 0, MapMode.Read, Vortice.Direct3D11.MapFlags.None, out var mapped).CheckError();
            try
            {
                StoreFrame(mapped.DataPointer, frame.ContentSize.Width, frame.ContentSize.Height, mapped.RowPitch);
            }
            finally
            {
                _d3dDevice.ImmediateContext.Unmap(_stagingTexture!, 0);
            }
        }
        catch (Exception ex)
        {
            lock (_frameLock)
            {
                _fatalError = ex;
            }

            _frameEvent.Set();
        }
    }

    private void EnsureStagingTexture(Texture2DDescription sourceDescription)
    {
        if (_stagingTexture is not null &&
            _stagingTexture.Description.Width == sourceDescription.Width &&
            _stagingTexture.Description.Height == sourceDescription.Height)
        {
            return;
        }

        _stagingTexture?.Dispose();
        var stagingDescription = new Texture2DDescription
        {
            Width = sourceDescription.Width,
            Height = sourceDescription.Height,
            MipLevels = 1,
            ArraySize = 1,
            Format = Vortice.DXGI.Format.B8G8R8A8_UNorm,
            SampleDescription = new Vortice.DXGI.SampleDescription(1, 0),
            Usage = ResourceUsage.Staging,
            BindFlags = BindFlags.None,
            CPUAccessFlags = CpuAccessFlags.Read,
            MiscFlags = ResourceOptionFlags.None
        };

        _stagingTexture = _d3dDevice!.CreateTexture2D(stagingDescription);
    }

    private void StoreFrame(IntPtr pixels, int width, int height, int sourcePitch)
    {
        var contiguousStride = width * 4;
        var requiredSize = contiguousStride * height;

        lock (_frameLock)
        {
            if (_frameBuffer is null || _frameBuffer.Length < requiredSize)
            {
                if (_frameBuffer is not null)
                {
                    ArrayPool<byte>.Shared.Return(_frameBuffer);
                }

                _frameBuffer = ArrayPool<byte>.Shared.Rent(requiredSize);
            }

            for (var y = 0; y < height; y++)
            {
                Marshal.Copy(
                    IntPtr.Add(pixels, y * sourcePitch),
                    _frameBuffer,
                    y * contiguousStride,
                    contiguousStride);
            }

            _frameWidth = width;
            _frameHeight = height;
            _frameStride = contiguousStride;
            _frameBytesWritten = requiredSize;
            _frameId++;
        }

        _frameEvent.Set();
    }

    private void ThrowIfDisposed()
    {
        if (_disposed)
        {
            throw new ObjectDisposedException(nameof(WgcCaptureService));
        }
    }

    private void ThrowIfFaulted()
    {
        lock (_frameLock)
        {
            if (_fatalError is not null)
            {
                throw new InvalidOperationException("Capture session failed.", _fatalError);
            }
        }
    }

    private static GraphicsCaptureItem CreateItemForWindow(IntPtr hwnd)
    {
        var factoryPointer = IntPtr.Zero;
        var itemPointer = IntPtr.Zero;
        var classIdHandle = IntPtr.Zero;

        try
        {
            const string classId = "Windows.Graphics.Capture.GraphicsCaptureItem";
            Marshal.ThrowExceptionForHR(WindowsCreateString(classId, classId.Length, out classIdHandle));
            var interopIid = GraphicsCaptureItemInteropIid;
            Marshal.ThrowExceptionForHR(RoGetActivationFactory(classIdHandle, ref interopIid, out factoryPointer));

            var interop = (IGraphicsCaptureItemInterop)Marshal.GetObjectForIUnknown(factoryPointer);
            var itemIid = GraphicsCaptureItemIid;
            Marshal.ThrowExceptionForHR(interop.CreateForWindow(hwnd, ref itemIid, out itemPointer));

            return MarshalInterface<GraphicsCaptureItem>.FromAbi(itemPointer);
        }
        finally
        {
            if (itemPointer != IntPtr.Zero)
            {
                Marshal.Release(itemPointer);
            }

            if (factoryPointer != IntPtr.Zero)
            {
                Marshal.Release(factoryPointer);
            }

            if (classIdHandle != IntPtr.Zero)
            {
                WindowsDeleteString(classIdHandle);
            }
        }
    }

    private static IDirect3DDevice CreateWinRtDevice(ID3D11Device d3dDevice)
    {
        using var dxgiDevice = d3dDevice.QueryInterface<Vortice.DXGI.IDXGIDevice>();
        var hr = CreateDirect3D11DeviceFromDXGIDevice(dxgiDevice.NativePointer, out var devicePointer);
        if (hr != 0)
        {
            Marshal.ThrowExceptionForHR((int)hr);
        }

        try
        {
            return MarshalInterface<IDirect3DDevice>.FromAbi(devicePointer);
        }
        finally
        {
            if (devicePointer != IntPtr.Zero)
            {
                Marshal.Release(devicePointer);
            }
        }
    }

    [DllImport("combase.dll", ExactSpelling = true)]
    private static extern int RoGetActivationFactory(
        IntPtr activatableClassId,
        [In] ref Guid iid,
        out IntPtr factory);

    [DllImport("combase.dll", ExactSpelling = true)]
    private static extern int WindowsCreateString(
        [MarshalAs(UnmanagedType.LPWStr)] string sourceString,
        int length,
        out IntPtr hstring);

    [DllImport("combase.dll", ExactSpelling = true)]
    private static extern int WindowsDeleteString(IntPtr hstring);

    [DllImport("d3d11.dll", EntryPoint = "D3D11CreateDevice", ExactSpelling = true)]
    private static extern int D3D11CreateDeviceNative(
        IntPtr adapter,
        DriverType driverType,
        IntPtr software,
        uint flags,
        [MarshalAs(UnmanagedType.LPArray, SizeParamIndex = 5)] FeatureLevel[]? featureLevels,
        uint featureLevelsCount,
        uint sdkVersion,
        out IntPtr device,
        out FeatureLevel featureLevel,
        out IntPtr immediateContext);

    [DllImport("d3d11.dll", EntryPoint = "CreateDirect3D11DeviceFromDXGIDevice", ExactSpelling = true)]
    private static extern uint CreateDirect3D11DeviceFromDXGIDevice(IntPtr dxgiDevice, out IntPtr graphicsDevice);

    [ComImport]
    [Guid("3628E81B-3CAC-4C60-B7F4-23CE0E0C3356")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IGraphicsCaptureItemInterop
    {
        int CreateForWindow([In] IntPtr window, [In] ref Guid iid, out IntPtr result);
    }

    [ComImport]
    [Guid("A9B3D012-3DF2-4EE3-B8D1-8695F457D3C1")]
    [InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
    private interface IDirect3DDxgiInterfaceAccess
    {
        IntPtr GetInterface([In] ref Guid iid);
    }

    private const uint D3D11SdkVersion = 7;
}

public enum CopyFrameStatus
{
    NoFrame = 0,
    Success = 1,
    BufferTooSmall = 2
}

public readonly record struct FrameCopyResult(
    CopyFrameStatus Status,
    int Width,
    int Height,
    int Stride,
    int BytesWritten,
    long FrameId)
{
    public static FrameCopyResult NoFrame => new(CopyFrameStatus.NoFrame, 0, 0, 0, 0, 0);
}
