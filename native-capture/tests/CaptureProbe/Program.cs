using System.Buffers.Binary;
using System.Text.Json;
using WindowCapture.Native;

if (args.Contains("--list", StringComparer.OrdinalIgnoreCase))
{
    Console.WriteLine(JsonSerializer.Serialize(
        WindowEnumeration.ListVisibleWindows().Select(window => new
        {
            handle = window.Handle,
            title = window.Title,
            process_id = window.ProcessId,
            process_name = window.ProcessName,
            class_name = window.ClassName,
        }),
        new JsonSerializerOptions
    {
        WriteIndented = true,
    }));
    return;
}

var hwnd = ReadRequired("--hwnd");
if (args.Contains("--stream", StringComparer.OrdinalIgnoreCase))
{
    await StreamFramesAsync((nint)long.Parse(hwnd));
    return;
}

if (args.Contains("--stdout-png", StringComparer.OrdinalIgnoreCase))
{
    var pngBytes = CaptureService.CaptureWindowToPngBytes((nint)long.Parse(hwnd));
    using var stdout = Console.OpenStandardOutput();
    stdout.Write(pngBytes, 0, pngBytes.Length);
    return;
}

var outputPath = ReadRequired("--out");
var result = await CaptureService.CaptureWindowToFileAsync((nint)long.Parse(hwnd), outputPath);

Console.WriteLine(JsonSerializer.Serialize(new
{
    path = result.Path,
    width = result.Width,
    height = result.Height,
    graphics_capture_supported = result.GraphicsCaptureSupported,
}));

static async Task StreamFramesAsync(nint hwnd)
{
    using var capture = new WgcCaptureService();
    capture.StartCapture(hwnd);

    using var stdout = Console.OpenStandardOutput();
    var header = new byte[24];
    var frameBuffer = Array.Empty<byte>();
    long lastSeenFrameId = 0;

    while (true)
    {
        var frameId = await Task.Run(() => capture.WaitForFrame(lastSeenFrameId, Timeout.Infinite));
        if (frameId <= 0)
        {
            continue;
        }

        lastSeenFrameId = frameId;

        while (true)
        {
            var probe = capture.CopyLatestFrame(IntPtr.Zero, 0);
            if (probe.Status == CopyFrameStatus.NoFrame)
            {
                break;
            }

            if (probe.BytesWritten <= 0)
            {
                throw new InvalidOperationException("Capture stream reported empty frame.");
            }

            if (frameBuffer.Length < probe.BytesWritten)
            {
                frameBuffer = new byte[probe.BytesWritten];
            }

            FrameCopyResult copyResult;
            unsafe
            {
                fixed (byte* destination = frameBuffer)
                {
                    copyResult = capture.CopyLatestFrame((IntPtr)destination, frameBuffer.Length);
                }
            }

            if (copyResult.Status == CopyFrameStatus.BufferTooSmall)
            {
                frameBuffer = new byte[copyResult.BytesWritten];
                continue;
            }

            if (copyResult.Status != CopyFrameStatus.Success)
            {
                break;
            }

            BinaryPrimitives.WriteUInt32LittleEndian(header.AsSpan(0, 4), (uint)copyResult.BytesWritten);
            BinaryPrimitives.WriteInt32LittleEndian(header.AsSpan(4, 4), copyResult.Width);
            BinaryPrimitives.WriteInt32LittleEndian(header.AsSpan(8, 4), copyResult.Height);
            BinaryPrimitives.WriteInt32LittleEndian(header.AsSpan(12, 4), copyResult.Stride);
            BinaryPrimitives.WriteInt64LittleEndian(header.AsSpan(16, 8), copyResult.FrameId);

            await stdout.WriteAsync(header, 0, header.Length);
            await stdout.WriteAsync(frameBuffer, 0, copyResult.BytesWritten);
            await stdout.FlushAsync();
            break;
        }
    }
}

string ReadRequired(string key)
{
    var index = Array.IndexOf(args, key);
    if (index < 0 || index + 1 >= args.Length)
    {
        throw new InvalidOperationException($"Missing argument {key}");
    }

    return args[index + 1];
}
