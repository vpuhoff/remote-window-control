namespace WindowCapture.Native;

public sealed record WindowInfo(
    long Handle,
    string Title,
    int ProcessId,
    string ProcessName,
    string ClassName);

public sealed record CaptureResult(
    string Path,
    int Width,
    int Height,
    bool GraphicsCaptureSupported);
