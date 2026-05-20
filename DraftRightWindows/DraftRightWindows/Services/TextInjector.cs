using System;
using System.Runtime.InteropServices;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Replaces the selected text in the source application by pasting from the clipboard.
/// Sets clipboard to the rewritten text, brings the target window to the foreground,
/// and simulates Ctrl+V using scan-code input (works in Electron/Chromium apps too).
/// </summary>
public sealed class TextInjector
{
    // ── Win32 imports ───────────────────────────────────────

    [DllImport("user32.dll")]
    private static extern bool SetForegroundWindow(IntPtr hWnd);

    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint lpdwProcessId);

    [DllImport("user32.dll")]
    private static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, bool fAttach);

    [DllImport("user32.dll")]
    private static extern bool BringWindowToTop(IntPtr hWnd);

    [DllImport("user32.dll")]
    private static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    private const int SW_RESTORE = 9;
    private const int SW_SHOW = 5;

    [DllImport("user32.dll")]
    private static extern bool IsIconic(IntPtr hWnd);

    [DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();

    [DllImport("user32.dll", SetLastError = true)]
    private static extern uint SendInput(uint nInputs, INPUT[] pInputs, int cbSize);

    // ── SendInput structs ───────────────────────────────────
    // The union MUST be sized to MOUSEINPUT (the largest member). Otherwise
    // Marshal.SizeOf<INPUT>() reports a smaller size than Win32 expects and
    // every SendInput call fails with ERROR_INVALID_PARAMETER (87).

    [StructLayout(LayoutKind.Sequential)]
    private struct INPUT
    {
        public uint type;
        public INPUTUNION union;
    }

    [StructLayout(LayoutKind.Explicit)]
    private struct INPUTUNION
    {
        [FieldOffset(0)] public MOUSEINPUT mi;
        [FieldOffset(0)] public KEYBDINPUT ki;
        [FieldOffset(0)] public HARDWAREINPUT hi;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct KEYBDINPUT
    {
        public ushort wVk;
        public ushort wScan;
        public uint dwFlags;
        public uint time;
        public IntPtr dwExtraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct MOUSEINPUT
    {
        public int dx;
        public int dy;
        public uint mouseData;
        public uint dwFlags;
        public uint time;
        public IntPtr dwExtraInfo;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct HARDWAREINPUT
    {
        public uint uMsg;
        public ushort wParamL;
        public ushort wParamH;
    }

    private const uint INPUT_KEYBOARD = 1;
    private const uint KEYEVENTF_KEYUP = 0x0002;
    private const uint KEYEVENTF_SCANCODE = 0x0008;

    // Scan codes (US layout — same on most layouts for these keys).
    private const ushort SC_LCTRL = 0x1D;
    private const ushort SC_V = 0x2F;

    // ── Dependencies ────────────────────────────────────────

    private readonly ClipboardService _clipboard;

    public TextInjector(ClipboardService clipboard)
    {
        _clipboard = clipboard;
    }

    // ── Public API ──────────────────────────────────────────

    /// <summary>
    /// Injects text into the target application by pasting from the clipboard.
    /// </summary>
    public async Task InjectTextAsync(string text, IntPtr targetWindow)
    {
        DRLogger.Log($"InjectText: target HWND=0x{targetWindow:X} len={text.Length}", DRLogger.Category.HOTKEY);

        // 1. Put the rewritten text on the clipboard.
        _clipboard.SetClipboardText(text);

        // 2. Bring the target window to the foreground reliably. SetForegroundWindow
        //    alone is silently rejected when the caller isn't already the foreground
        //    process. The standard workaround is to attach our thread's input queue
        //    to the target window's thread first.
        if (targetWindow != IntPtr.Zero)
        {
            if (IsIconic(targetWindow)) ShowWindow(targetWindow, SW_RESTORE);

            uint targetThread = GetWindowThreadProcessId(targetWindow, out _);
            uint thisThread = GetCurrentThreadId();
            bool attached = false;
            try
            {
                if (targetThread != 0 && targetThread != thisThread)
                    attached = AttachThreadInput(thisThread, targetThread, true);

                BringWindowToTop(targetWindow);
                SetForegroundWindow(targetWindow);
                ShowWindow(targetWindow, SW_SHOW);
            }
            finally
            {
                if (attached) AttachThreadInput(thisThread, targetThread, false);
            }
        }

        // 3. Wait for the foreground switch to land. Apps queue input until they
        //    have foreground focus, so under-waiting drops the paste.
        await Task.Delay(150);

        // 4. Simulate Ctrl+V using SCAN CODES — works in Electron/Chromium apps
        //    (VS Code, browsers) where VK-only SendInput is filtered out.
        var sent = SimulateCtrlVViaScanCode();
        if (sent != 4)
        {
            int err = Marshal.GetLastWin32Error();
            DRLogger.Warn($"InjectText: SendInput sent {sent}/4, GetLastError={err}", DRLogger.Category.HOTKEY);
        }

        // 5. Brief delay for the paste to register before our caller continues.
        await Task.Delay(80);
        DRLogger.Log("InjectText: done", DRLogger.Category.HOTKEY);
    }

    /// <summary>
    /// Gets the current foreground window handle.
    /// Call this before showing the DraftRight UI so you know where to paste back.
    /// </summary>
    public static IntPtr GetCurrentForegroundWindow() => GetForegroundWindow();

    // ── Private helpers ─────────────────────────────────────

    private static uint SimulateCtrlVViaScanCode()
    {
        var inputs = new INPUT[]
        {
            MakeScanCodeInput(SC_LCTRL, keyUp: false),
            MakeScanCodeInput(SC_V,     keyUp: false),
            MakeScanCodeInput(SC_V,     keyUp: true),
            MakeScanCodeInput(SC_LCTRL, keyUp: true),
        };
        return SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
    }

    private static INPUT MakeScanCodeInput(ushort scan, bool keyUp)
    {
        uint flags = KEYEVENTF_SCANCODE;
        if (keyUp) flags |= KEYEVENTF_KEYUP;

        return new INPUT
        {
            type = INPUT_KEYBOARD,
            union = new INPUTUNION
            {
                ki = new KEYBDINPUT
                {
                    wVk = 0,
                    wScan = scan,
                    dwFlags = flags,
                    time = 0,
                    dwExtraInfo = IntPtr.Zero,
                }
            }
        };
    }
}
