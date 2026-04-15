using System;
using System.Runtime.InteropServices;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Replaces the selected text in the source application by pasting from the clipboard.
/// Sets clipboard to the rewritten text, brings the target window to the foreground,
/// and simulates Ctrl+V.
/// </summary>
public sealed class TextInjector
{
    // ── Win32 imports ───────────────────────────────────────

    [DllImport("user32.dll")]
    private static extern bool SetForegroundWindow(IntPtr hWnd);

    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll", SetLastError = true)]
    private static extern uint SendInput(uint nInputs, INPUT[] pInputs, int cbSize);

    // ── SendInput structs (mirrors ClipboardService) ────────

    [StructLayout(LayoutKind.Sequential)]
    private struct INPUT
    {
        public uint type;
        public INPUTUNION union;
    }

    [StructLayout(LayoutKind.Explicit)]
    private struct INPUTUNION
    {
        [FieldOffset(0)] public KEYBDINPUT ki;
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

    private const uint INPUT_KEYBOARD = 1;
    private const uint KEYEVENTF_KEYUP = 0x0002;
    private const ushort VK_CONTROL = 0x11;
    private const ushort VK_V = 0x56;

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
    /// <param name="text">The rewritten text to inject.</param>
    /// <param name="targetWindow">Handle of the window that should receive the paste.</param>
    public async Task InjectTextAsync(string text, IntPtr targetWindow)
    {
        // 1. Place the rewritten text on the clipboard
        _clipboard.SetClipboardText(text);

        // 2. Bring the target window to the foreground
        SetForegroundWindow(targetWindow);

        // 3. Small delay to let the OS finish the window switch
        await Task.Delay(100);

        // 4. Simulate Ctrl+V to paste
        SimulateCtrlV();

        // 5. Brief delay for the paste to register
        await Task.Delay(50);
    }

    /// <summary>
    /// Gets the current foreground window handle.
    /// Call this before showing the DraftRight UI so you know where to paste back.
    /// </summary>
    public static IntPtr GetCurrentForegroundWindow() => GetForegroundWindow();

    // ── Private helpers ─────────────────────────────────────

    private static void SimulateCtrlV()
    {
        var inputs = new INPUT[]
        {
            MakeKeyInput(VK_CONTROL, keyUp: false),
            MakeKeyInput(VK_V, keyUp: false),
            MakeKeyInput(VK_V, keyUp: true),
            MakeKeyInput(VK_CONTROL, keyUp: true),
        };

        SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
    }

    private static INPUT MakeKeyInput(ushort vk, bool keyUp)
    {
        return new INPUT
        {
            type = INPUT_KEYBOARD,
            union = new INPUTUNION
            {
                ki = new KEYBDINPUT
                {
                    wVk = vk,
                    wScan = 0,
                    dwFlags = keyUp ? KEYEVENTF_KEYUP : 0,
                    time = 0,
                    dwExtraInfo = IntPtr.Zero
                }
            }
        };
    }
}
