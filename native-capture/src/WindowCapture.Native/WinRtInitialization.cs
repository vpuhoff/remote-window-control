using WinRT;

namespace WindowCapture.Native;

internal static class WinRtInitialization
{
    private static int _initialized;

    public static void EnsureInitialized()
    {
        if (Interlocked.Exchange(ref _initialized, 1) != 0)
        {
            return;
        }

        ComWrappersSupport.InitializeComWrappers();
    }
}
