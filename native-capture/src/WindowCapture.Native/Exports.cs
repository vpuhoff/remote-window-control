using System.Runtime.InteropServices;

namespace WindowCapture.Native;

public static unsafe class Exports
{
    [UnmanagedCallersOnly(EntryPoint = "Capture_IsGraphicsCaptureSupported")]
    public static int IsGraphicsCaptureSupported() => CaptureService.IsGraphicsCaptureSupported() ? 1 : 0;

    [UnmanagedCallersOnly(EntryPoint = "Capture_GetVisibleWindowCount")]
    public static int GetVisibleWindowCount() => WindowEnumeration.ListVisibleWindows().Count;

    [UnmanagedCallersOnly(EntryPoint = "Capture_SaveWindowSnapshot")]
    public static int SaveWindowSnapshot(nint hwnd, char* outputPath, int outputLength)
    {
        try
        {
            var path = new string(outputPath, 0, outputLength);
            CaptureService.CaptureWindowToFile(hwnd, path);
            return 0;
        }
        catch
        {
            return -1;
        }
    }
}
