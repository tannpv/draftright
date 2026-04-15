using System.Runtime.InteropServices;

namespace DraftRightWindows.Helpers;

/// <summary>
/// P/Invoke declarations for Win32 APIs used by DraftRight:
/// hotkey registration, foreground window management, and synthetic keyboard input.
/// </summary>
public static class Win32Interop
{
    // ── Hotkey registration ──

    [DllImport("user32.dll", SetLastError = true)]
    public static extern bool RegisterHotKey(IntPtr hWnd, int id, uint fsModifiers, uint vk);

    [DllImport("user32.dll", SetLastError = true)]
    public static extern bool UnregisterHotKey(IntPtr hWnd, int id);

    // ── Window visibility ──

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

    public const int SW_HIDE = 0;
    public const int SW_SHOW = 5;

    // ── Foreground window ──

    [DllImport("user32.dll")]
    public static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool SetForegroundWindow(IntPtr hWnd);

    // ── Synthetic keyboard input ──

    [DllImport("user32.dll", SetLastError = true)]
    public static extern uint SendInput(uint nInputs, INPUT[] pInputs, int cbSize);

    // ── Modifier key constants ──

    public const uint MOD_NONE = 0x0000;
    public const uint MOD_ALT = 0x0001;
    public const uint MOD_CONTROL = 0x0002;
    public const uint MOD_SHIFT = 0x0004;
    public const uint MOD_WIN = 0x0008;
    public const uint MOD_NOREPEAT = 0x4000;

    // ── Virtual key codes ──

    public const uint VK_BACK = 0x08;
    public const uint VK_TAB = 0x09;
    public const uint VK_RETURN = 0x0D;
    public const uint VK_SHIFT = 0x10;
    public const uint VK_CONTROL = 0x11;
    public const uint VK_MENU = 0x12;      // Alt
    public const uint VK_ESCAPE = 0x1B;
    public const uint VK_SPACE = 0x20;

    // Letters (A-Z are 0x41-0x5A)
    public const uint VK_A = 0x41;
    public const uint VK_C = 0x43;
    public const uint VK_D = 0x44;
    public const uint VK_R = 0x52;
    public const uint VK_V = 0x56;
    public const uint VK_X = 0x58;
    public const uint VK_Z = 0x5A;

    // Function keys
    public const uint VK_F1 = 0x70;
    public const uint VK_F2 = 0x71;
    public const uint VK_F3 = 0x72;
    public const uint VK_F4 = 0x73;
    public const uint VK_F5 = 0x74;
    public const uint VK_F12 = 0x7B;

    // ── Keyboard event flags ──

    public const uint KEYEVENTF_KEYDOWN = 0x0000;
    public const uint KEYEVENTF_EXTENDEDKEY = 0x0001;
    public const uint KEYEVENTF_KEYUP = 0x0002;
    public const uint KEYEVENTF_UNICODE = 0x0004;

    // ── Input type constants ──

    public const uint INPUT_MOUSE = 0;
    public const uint INPUT_KEYBOARD = 1;
    public const uint INPUT_HARDWARE = 2;

    // ── Helper: simulate a key-chord (e.g., Ctrl+C, Ctrl+V) ──

    public static void SimulateKeyChord(uint modifierVk, uint keyVk)
    {
        var inputs = new INPUT[]
        {
            // Modifier key down
            CreateKeyInput(modifierVk, KEYEVENTF_KEYDOWN),
            // Main key down
            CreateKeyInput(keyVk, KEYEVENTF_KEYDOWN),
            // Main key up
            CreateKeyInput(keyVk, KEYEVENTF_KEYUP),
            // Modifier key up
            CreateKeyInput(modifierVk, KEYEVENTF_KEYUP),
        };

        SendInput((uint)inputs.Length, inputs, Marshal.SizeOf<INPUT>());
    }

    /// <summary>Convenience: simulate Ctrl+C to copy selected text.</summary>
    public static void SimulateCopy() => SimulateKeyChord(VK_CONTROL, VK_C);

    /// <summary>Convenience: simulate Ctrl+V to paste from clipboard.</summary>
    public static void SimulatePaste() => SimulateKeyChord(VK_CONTROL, VK_V);

    private static INPUT CreateKeyInput(uint vk, uint flags)
    {
        return new INPUT
        {
            type = INPUT_KEYBOARD,
            u = new InputUnion
            {
                ki = new KEYBDINPUT
                {
                    wVk = (ushort)vk,
                    wScan = 0,
                    dwFlags = flags,
                    time = 0,
                    dwExtraInfo = IntPtr.Zero
                }
            }
        };
    }
}

// ── Structs for SendInput ──

[StructLayout(LayoutKind.Sequential)]
public struct INPUT
{
    public uint type;
    public InputUnion u;
}

[StructLayout(LayoutKind.Explicit)]
public struct InputUnion
{
    [FieldOffset(0)] public MOUSEINPUT mi;
    [FieldOffset(0)] public KEYBDINPUT ki;
    [FieldOffset(0)] public HARDWAREINPUT hi;
}

[StructLayout(LayoutKind.Sequential)]
public struct KEYBDINPUT
{
    public ushort wVk;
    public ushort wScan;
    public uint dwFlags;
    public uint time;
    public IntPtr dwExtraInfo;
}

[StructLayout(LayoutKind.Sequential)]
public struct MOUSEINPUT
{
    public int dx;
    public int dy;
    public uint mouseData;
    public uint dwFlags;
    public uint time;
    public IntPtr dwExtraInfo;
}

[StructLayout(LayoutKind.Sequential)]
public struct HARDWAREINPUT
{
    public uint uMsg;
    public ushort wParamL;
    public ushort wParamH;
}
