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
        DRLogger.Log(
            $"Register: hwnd=0x{hwnd.ToInt64():X} modifiers=0x{modifiers:X} vk=0x{vk:X}",
            DRLogger.Category.HOTKEY);

        if (_isRegistered)
        {
            DRLogger.Log("Register: replacing existing registration", DRLogger.Category.HOTKEY);
            Unregister();
        }

        _isRegistered = RegisterHotKey(hwnd, HOTKEY_ID, modifiers, vk);
        if (_isRegistered)
        {
            _registeredHwnd = hwnd;
            DRLogger.Log($"Register: succeeded id=0x{HOTKEY_ID:X}", DRLogger.Category.HOTKEY);
        }
        else
        {
            var lastError = Marshal.GetLastWin32Error();
            DRLogger.Log(
                $"Register: FAILED — RegisterHotKey returned false, GetLastError=0x{lastError:X} ({lastError})",
                DRLogger.Category.HOTKEY);
        }

        return _isRegistered;
    }

    /// <summary>
    /// Registers the default hotkey (Ctrl+Shift+R).
    /// </summary>
    public bool RegisterDefault(IntPtr hwnd)
    {
        DRLogger.Log("RegisterDefault: Ctrl+Shift+R", DRLogger.Category.HOTKEY);
        return Register(hwnd, MOD_CONTROL | MOD_SHIFT, VK_R);
    }

    /// <summary>
    /// Unregisters the current hotkey.
    /// </summary>
    public void Unregister()
    {
        if (!_isRegistered)
        {
            DRLogger.Log("Unregister: no-op (not registered)", DRLogger.Category.HOTKEY);
            return;
        }

        var ok = UnregisterHotKey(_registeredHwnd, HOTKEY_ID);
        DRLogger.Log(
            $"Unregister: hwnd=0x{_registeredHwnd.ToInt64():X} returned={ok}",
            DRLogger.Category.HOTKEY);
        _registeredHwnd = IntPtr.Zero;
        _isRegistered = false;
    }

    /// <summary>
    /// Call this from the window procedure when WM_HOTKEY is received.
    /// </summary>
    public void ProcessHotkeyMessage(int hotkeyId)
    {
        if (hotkeyId == HOTKEY_ID)
        {
            DRLogger.Log("ProcessHotkeyMessage: WM_HOTKEY received, firing HotkeyPressed",
                DRLogger.Category.HOTKEY);
            try
            {
                HotkeyPressed?.Invoke(this, EventArgs.Empty);
            }
            catch (Exception ex)
            {
                DRLogger.Log(
                    $"ProcessHotkeyMessage: handler threw — {ex.GetType().Name}: {ex.Message}",
                    DRLogger.Category.HOTKEY);
                throw;
            }
        }
        else
        {
            DRLogger.Log(
                $"ProcessHotkeyMessage: id=0x{hotkeyId:X} did not match HOTKEY_ID 0x{HOTKEY_ID:X} — ignored",
                DRLogger.Category.HOTKEY);
        }
    }

    public void Dispose() => Unregister();
}
