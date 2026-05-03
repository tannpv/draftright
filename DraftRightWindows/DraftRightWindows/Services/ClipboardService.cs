using System;
using System.Runtime.InteropServices;
using System.Threading;
using System.Threading.Tasks;

namespace DraftRightWindows.Services;

/// <summary>
/// Clipboard operations: read selected text via simulated Ctrl+C, set clipboard content.
/// Uses Win32 SendInput and clipboard APIs.
/// </summary>
public sealed class ClipboardService
{
    // ── Win32 imports ───────────────────────────────────────

    [DllImport("user32.dll", SetLastError = true)]
    private static extern uint SendInput(uint nInputs, INPUT[] pInputs, int cbSize);

    [DllImport("user32.dll")]
    private static extern bool OpenClipboard(IntPtr hWndNewOwner);

    [DllImport("user32.dll")]
    private static extern bool CloseClipboard();

    [DllImport("user32.dll")]
    private static extern bool EmptyClipboard();

    [DllImport("user32.dll")]
    private static extern IntPtr GetClipboardData(uint uFormat);

    [DllImport("user32.dll")]
    private static extern IntPtr SetClipboardData(uint uFormat, IntPtr hMem);

    [DllImport("kernel32.dll")]
    private static extern IntPtr GlobalAlloc(uint uFlags, UIntPtr dwBytes);

    [DllImport("kernel32.dll")]
    private static extern IntPtr GlobalLock(IntPtr hMem);

    [DllImport("kernel32.dll")]
    private static extern bool GlobalUnlock(IntPtr hMem);

    [DllImport("kernel32.dll")]
    private static extern UIntPtr GlobalSize(IntPtr hMem);

    // ── Constants ───────────────────────────────────────────

    private const uint CF_UNICODETEXT = 13;
    private const uint GMEM_MOVEABLE = 0x0002;

    private const ushort VK_CONTROL  = 0x11;
    private const ushort VK_LCONTROL = 0xA2;
    private const ushort VK_RCONTROL = 0xA3;
    private const ushort VK_SHIFT    = 0x10;
    private const ushort VK_LSHIFT   = 0xA0;
    private const ushort VK_RSHIFT   = 0xA1;
    private const ushort VK_MENU     = 0x12; // Alt
    private const ushort VK_LMENU    = 0xA4;
    private const ushort VK_RMENU    = 0xA5;
    private const ushort VK_LWIN     = 0x5B;
    private const ushort VK_RWIN     = 0x5C;
    private const ushort VK_C        = 0x43;
    private const ushort VK_V = 0x56;

    private const uint KEYEVENTF_KEYUP = 0x0002;

    // ── SendInput structs ───────────────────────────────────

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

    // ── Public API ──────────────────────────────────────────

    /// <summary>
    /// Captures the currently selected text in the foreground application.
    /// Saves/restores the existing clipboard content around the operation.
    /// </summary>
    public async Task<string?> GetSelectedTextAsync()
    {
        var fg0 = NativeForegroundWindow();
        DRLogger.Log($"GetSelectedTextAsync start fg=0x{fg0:X}", DRLogger.Category.HOTKEY);

        // 1. Save current clipboard content
        string? originalClipboard = ReadClipboardText();
        DRLogger.Log($"  step1: originalClipboard len={(originalClipboard?.Length ?? -1)}",
            DRLogger.Category.HOTKEY);

        // 2. Clear clipboard so we can detect new content
        ClearClipboard();

        // 3. Release any modifier keys the user may still be holding from
        //    the global hotkey (Ctrl+Shift+R typically). Otherwise our
        //    SimulateKeyCombo(Ctrl, C) below ends up effectively
        //    Ctrl+Shift+C from the OS's perspective.
        ReleaseHeldModifiers();
        await Task.Delay(50);

        var fg1 = NativeForegroundWindow();
        DRLogger.Log($"  step3: fg=0x{fg1:X} (after modifier release)",
            DRLogger.Category.HOTKEY);

        // 4. Simulate clean Ctrl+C
        var (sent, err) = SimulateKeyComboReporting(VK_CONTROL, VK_C);
        DRLogger.Log($"  step4: SendInput Ctrl+C → events sent={sent} lastError={err} (0x{err:X})",
            DRLogger.Category.HOTKEY);

        // 5. Wait, then poll the clipboard up to 8x100ms in case the app
        //    is slow to populate it (web/Electron editors often are).
        string? selectedText = null;
        for (int attempt = 1; attempt <= 8; attempt++)
        {
            await Task.Delay(100);
            selectedText = ReadClipboardText();
            DRLogger.Log($"  step5 attempt {attempt}: clipboard len={(selectedText?.Length ?? -1)}",
                DRLogger.Category.HOTKEY);
            if (!string.IsNullOrEmpty(selectedText))
                break;
        }

        // 6. Restore original clipboard
        if (originalClipboard != null)
            SetClipboardText(originalClipboard);

        return selectedText;
    }

    /// <summary>
    /// Same as SimulateKeyCombo but returns (sent, lastError) for diagnosis.
    /// SendInput returning 0 means UIPI/UAC blocked us, BlockInput is active,
    /// or the target desktop differs.
    /// </summary>
    private (uint sent, int lastError) SimulateKeyComboReporting(params ushort[] vkCodes)
    {
        var inputs = new INPUT[vkCodes.Length * 2];
        int idx = 0;
        foreach (var vk in vkCodes) inputs[idx++] = MakeKeyInput(vk, keyUp: false);
        for (int i = vkCodes.Length - 1; i >= 0; i--) inputs[idx++] = MakeKeyInput(vkCodes[i], keyUp: true);
        var sent = SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
        var err = sent == 0 ? Marshal.GetLastWin32Error() : 0;
        return (sent, err);
    }

    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();

    private static long NativeForegroundWindow() => GetForegroundWindow().ToInt64();

    /// <summary>
    /// Force keyup events for every modifier the user might be holding
    /// down from a global-hotkey trigger. Standard pattern for capturing
    /// selection after a hotkey fires.
    /// </summary>
    private static void ReleaseHeldModifiers()
    {
        ushort[] mods =
        {
            VK_LCONTROL, VK_RCONTROL, VK_CONTROL,
            VK_LSHIFT,   VK_RSHIFT,   VK_SHIFT,
            VK_LMENU,    VK_RMENU,    VK_MENU,
            VK_LWIN,     VK_RWIN,
        };
        var inputs = new INPUT[mods.Length];
        for (int i = 0; i < mods.Length; i++)
        {
            inputs[i] = MakeKeyInput(mods[i], keyUp: true);
        }
        SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
    }

    /// <summary>
    /// Sets the system clipboard to the specified text.
    /// </summary>
    public void SetClipboardText(string text)
    {
        if (!OpenClipboard(IntPtr.Zero))
            return;

        try
        {
            EmptyClipboard();

            var bytes = (text.Length + 1) * 2; // UTF-16 + null terminator
            var hGlobal = GlobalAlloc(GMEM_MOVEABLE, (UIntPtr)bytes);
            if (hGlobal == IntPtr.Zero)
                return;

            var ptr = GlobalLock(hGlobal);
            if (ptr == IntPtr.Zero)
                return;

            Marshal.Copy(text.ToCharArray(), 0, ptr, text.Length);
            // Null terminator (2 bytes of zero) is already there from GlobalAlloc zeroing
            GlobalUnlock(hGlobal);

            SetClipboardData(CF_UNICODETEXT, hGlobal);
            // Do NOT free hGlobal — ownership transfers to the clipboard
        }
        finally
        {
            CloseClipboard();
        }
    }

    /// <summary>
    /// Simulates a key combination (e.g., Ctrl+C) via SendInput.
    /// </summary>
    public void SimulateKeyCombo(params ushort[] vkCodes)
    {
        var inputs = new INPUT[vkCodes.Length * 2];
        int idx = 0;

        // Key-down in order
        foreach (var vk in vkCodes)
        {
            inputs[idx++] = MakeKeyInput(vk, keyUp: false);
        }

        // Key-up in reverse order
        for (int i = vkCodes.Length - 1; i >= 0; i--)
        {
            inputs[idx++] = MakeKeyInput(vkCodes[i], keyUp: true);
        }

        SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
    }

    // ── Private helpers ─────────────────────────────────────

    private static string? ReadClipboardText()
    {
        if (!OpenClipboard(IntPtr.Zero))
            return null;

        try
        {
            var hData = GetClipboardData(CF_UNICODETEXT);
            if (hData == IntPtr.Zero)
                return null;

            var ptr = GlobalLock(hData);
            if (ptr == IntPtr.Zero)
                return null;

            try
            {
                return Marshal.PtrToStringUni(ptr);
            }
            finally
            {
                GlobalUnlock(hData);
            }
        }
        finally
        {
            CloseClipboard();
        }
    }

    private static void ClearClipboard()
    {
        if (!OpenClipboard(IntPtr.Zero))
            return;

        EmptyClipboard();
        CloseClipboard();
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
