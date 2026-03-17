using SixLabors.ImageSharp;
using SixLabors.ImageSharp.Formats.Png;
using SixLabors.ImageSharp.PixelFormats;

namespace WindowCapture.Native;

public static class CaptureService
{
    private static readonly bool DebugLoggingEnabled =
        string.Equals(Environment.GetEnvironmentVariable("WINDOW_CAPTURE_DEBUG"), "1", StringComparison.Ordinal);

    public static bool IsGraphicsCaptureSupported() => OperatingSystem.IsWindowsVersionAtLeast(10, 0, 18362);

    public static Task<CaptureResult> CaptureWindowToFileAsync(nint hwnd, string outputPath, CancellationToken cancellationToken = default)
    {
        cancellationToken.ThrowIfCancellationRequested();
        return Task.Run(() => CaptureWindowToFile(hwnd, outputPath), cancellationToken);
    }

    public static CaptureResult CaptureWindowToFile(nint hwnd, string outputPath)
    {
        using var image = CaptureWindowImage(hwnd, out var width, out var height);
        LogDebug($"Saving PNG to {outputPath}");
        Directory.CreateDirectory(Path.GetDirectoryName(outputPath) ?? ".");
        image.SaveAsPng(outputPath);
        LogDebug("Saved PNG to disk");
        return new CaptureResult(outputPath, width, height, IsGraphicsCaptureSupported());
    }

    public static byte[] CaptureWindowToPngBytes(nint hwnd)
    {
        using var image = CaptureWindowImage(hwnd, out _, out _);
        using var stream = new MemoryStream();
        LogDebug("Encoding PNG to memory");
        image.Save(stream, new PngEncoder());
        LogDebug("Encoded PNG to memory");
        return stream.ToArray();
    }

    private static Image<Bgra32> CaptureWindowImage(nint hwnd, out int width, out int height)
    {
        LogDebug($"Capture start hwnd={hwnd}");
        if (!IsGraphicsCaptureSupported())
        {
            throw new PlatformNotSupportedException("Windows.Graphics.Capture требует Windows 10 1903+.");
        }

        using var capture = new WgcCaptureService();
        capture.StartCapture(hwnd);

        var frameId = capture.WaitForFrame(0, 3000);
        if (frameId <= 0)
        {
            throw new TimeoutException("Не удалось получить первый кадр захвата.");
        }

        var copyResult = capture.CopyLatestFrame(IntPtr.Zero, 0);
        if (copyResult.Status == CopyFrameStatus.NoFrame || copyResult.BytesWritten <= 0)
        {
            throw new InvalidOperationException("Кадр захвата недоступен.");
        }

        width = copyResult.Width;
        height = copyResult.Height;
        var buffer = new byte[copyResult.BytesWritten];
        unsafe
        {
            fixed (byte* destination = buffer)
            {
                copyResult = capture.CopyLatestFrame((IntPtr)destination, buffer.Length);
            }
        }

        if (copyResult.Status != CopyFrameStatus.Success)
        {
            throw new InvalidOperationException($"Не удалось скопировать кадр захвата: {copyResult.Status}.");
        }

        return Image.LoadPixelData<Bgra32>(buffer.AsSpan(0, copyResult.BytesWritten).ToArray(), width, height);
    }

    private static void LogDebug(string message)
    {
        if (DebugLoggingEnabled)
        {
            Console.Error.WriteLine($"[capture] {message}");
        }
    }
}
