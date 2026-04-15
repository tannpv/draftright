using System;
using System.Runtime.InteropServices;

namespace DraftRightWindows.Services;

/// <summary>
/// Registers and manages a system-wide global hotkey using Win32 RegisterHotKey.
/// The app's hidden message-only window processes WM_HOTKEY and calls <see cref="HotkeyPressed"/>.
/// </summary>
public sealed class HotkeyService : IDisposable
{
    // ── Win32 imports ───────────────────────────────────────

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool RegisterHotKey(IntPtr hWnd, int id, uint fsModifiers, uint vk);

    [DllImport("user32.dll", SetLastError = true)]
    private static extern bool UnregisterHotKey(IntPtr hWnd, int id);

    // ── Constants ───────────────────────────────────────────

    /// <summary>WM_HOTKEY message ID.</summary>
    public const int WM_HOTKEY = 0x0312;

    /// <summary>Hotkey registration atom ID (arbitrary, must be unique per window).</summary>
    private const int HOTKEY_ID = 0xD001;

    // Modifier flags (same as Win32Interop)
    public const uint MOD_ALT     = 0x0001;
    public const uint MOD_CONTROL = 0x0002;
    public const uint MOD_SHIFT   = 0x0004;
    public const uint MOD_WIN     = 0x0008;

    // Default key: R
    public const uint VK_R = 0x52;

    // ── State ───────────────────────────────────────────────

    private IntPtr _registeredHwnd = IntPtr.Zero;
    private bool _isRegistered;

    /// <summary>
    /// Raised when the registered global hotkey is pressed.
    /// </summary>
    public event EventHandler? HotkeyPressed;

    // ── Public API ──────────────────────────────────────────

    /// <summary>
    /// Registers a global hotkey on the given window handle.
    /// </summary>
    /// <param name="hwnd">Handle of the message-only window that will receive WM_HOTKEY.</param>
    /// <param name="modifiers">Combination of MOD_* flags (e.g., MOD_CONTROL | MOD_SHIFT).</param>
    /// <param name="vk">Virtual-key code (e.g., VK_R).</param>
    /// <returns>True if registration succeeded.</returns>
    public bool Register(IntPtr hwnd, uint modifiers, uint vk)
    {
        if (_isRegistered)
            Unregister();

        _isRegistered = RegisterHotKey(hwnd, HOTKEY_ID, modifiers, vk);
        if (_isRegistered)
            _registeredHwnd = hwnd;

        return _isRegistered;
    }

    /// <summary>
    /// Registers the default hotkey (Ctrl+Shift+R).
    /// </summary>
    public bool RegisterDefault(IntPtr hwnd)
    {
        return Register(hwnd, MOD_CONTROL | MOD_SHIFT, VK_R);
    }

    /// <summary>
    /// Unregisters the current hotkey.
    /// </summary>
    public void Unregister()
    {
        if (!_isRegistered)
            return;

        UnregisterHotKey(_registeredHwnd, HOTKEY_ID);
        _registeredHwnd = IntPtr.Zero;
        _isRegistered = false;
    }

    /// <summary>
    /// Call this from the window procedure when WM_HOTKEY is received.
    /// </summary>
    public void ProcessHotkeyMessage(int hotkeyId)
    {
        if (hotkeyId == HOTKEY_ID)
            HotkeyPressed?.Invoke(this, EventArgs.Empty);
    }

    public void Dispose() => Unregister();
}
