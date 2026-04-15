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

    private const ushort VK_CONTROL = 0x11;
    private const ushort VK_C = 0x43;
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
        // 1. Save current clipboard content
        string? originalClipboard = ReadClipboardText();

        // 2. Clear clipboard so we can detect new content
        ClearClipboard();

        // 3. Simulate Ctrl+C
        SimulateKeyCombo(VK_CONTROL, VK_C);

        // 4. Wait for the copy to complete
        await Task.Delay(100);

        // 5. Read the newly copied text
        string? selectedText = ReadClipboardText();

        // 6. Restore original clipboard
        if (originalClipboard != null)
            SetClipboardText(originalClipboard);

        return selectedText;
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
