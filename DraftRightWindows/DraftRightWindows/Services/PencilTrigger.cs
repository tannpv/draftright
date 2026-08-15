using System;
using System.Threading;
using WinForms = System.Windows.Forms;
using DraftRightWindows.Helpers;
using DraftRightWindows.Views;

namespace DraftRightWindows.Services;

/// <summary>
/// The Windows on-selection pencil trigger (DraftRight #180) — the counterpart of
/// the macOS SelectionMonitor. Installs a global low-level mouse hook on its own
/// STA message-pump thread, detects a drag-highlight, and shows a floating
/// <see cref="PencilOverlayForm"/> near the drag end. Clicking the pencil raises
/// <c>onTriggered</c>; the app then grabs the selection (Ctrl+C) and rewrites —
/// the same flow the hotkey uses.
///
/// Deliberately gesture-only: it never touches the clipboard on selection (that
/// broke the user's own Copy on macoS, #178) — Ctrl+C happens only on the
/// deliberate pencil click, in the app's trigger handler.
///
/// UNVERIFIED on Windows as of authoring — global mouse hooks, cross-thread UI,
/// and NOACTIVATE focus behavior can only be validated on a real machine. Logs
/// verbosely (category APP, "Pencil:") so the runtime log drives debugging.
/// </summary>
public sealed class PencilTrigger
{
    private const int DragThresholdPx = 4;

    private readonly Action _onTriggered;

    private Thread? _thread;
    private PencilOverlayForm? _overlay;
    private IntPtr _hook = IntPtr.Zero;
    private Win32Interop.LowLevelMouseProc? _proc; // MUST stay alive while hooked
    private volatile bool _enabled;

    // Drag state — touched only on the pump thread.
    private bool _down;
    private bool _dragging;
    private Win32Interop.POINT _downPt;

    public PencilTrigger(Action onTriggered) => _onTriggered = onTriggered;

    /// <summary>Enable or disable the pencil. Idempotent. Called from ApplyTriggerMode.</summary>
    public void SetEnabled(bool enabled)
    {
        if (enabled == _enabled) return;
        _enabled = enabled;
        if (enabled) Start();
        else Stop();
    }

    private void Start()
    {
        if (_thread != null) return;
        _thread = new Thread(ThreadMain) { IsBackground = true, Name = "PencilTrigger" };
        _thread.SetApartmentState(ApartmentState.STA);
        _thread.Start();
    }

    private void ThreadMain()
    {
        try
        {
            _overlay = new PencilOverlayForm();
            _overlay.PencilClicked += OnPencilClicked;
            _ = _overlay.Handle; // force handle creation so BeginInvoke works

            _proc = HookProc;
            _hook = Win32Interop.SetWindowsHookEx(
                Win32Interop.WH_MOUSE_LL, _proc, Win32Interop.GetModuleHandle(null), 0);
            DRLogger.Log($"Pencil: hook installed={_hook != IntPtr.Zero}", DRLogger.Category.APP);

            WinForms.Application.Run(); // pumps the hook + the overlay
        }
        catch (Exception ex)
        {
            DRLogger.Error($"Pencil: thread crashed: {ex.GetType().Name}: {ex.Message}", DRLogger.Category.APP);
        }
        finally
        {
            if (_hook != IntPtr.Zero) { Win32Interop.UnhookWindowsHookEx(_hook); _hook = IntPtr.Zero; }
            try { _overlay?.Dispose(); } catch { /* ignore */ }
            _overlay = null;
            _proc = null;
        }
    }

    private void Stop()
    {
        var overlay = _overlay;
        _thread = null;
        if (overlay == null) return;
        try
        {
            overlay.BeginInvoke(new Action(() =>
            {
                if (_hook != IntPtr.Zero) { Win32Interop.UnhookWindowsHookEx(_hook); _hook = IntPtr.Zero; }
                WinForms.Application.ExitThread(); // ends Application.Run → ThreadMain finally cleans up
            }));
        }
        catch (Exception ex)
        {
            DRLogger.Warn($"Pencil: stop failed: {ex.Message}", DRLogger.Category.APP);
        }
    }

    // Runs on the pump thread. MUST be fast — it is on the global input path.
    private IntPtr HookProc(int nCode, IntPtr wParam, IntPtr lParam)
    {
        if (nCode == Win32Interop.HC_ACTION)
        {
            switch (wParam.ToInt32())
            {
                case Win32Interop.WM_LBUTTONDOWN:
                    _down = true;
                    _dragging = false;
                    Win32Interop.GetCursorPos(out _downPt);
                    // A new click that is NOT on the pencil dismisses it.
                    if (!CursorOverOverlay()) HideOverlay();
                    break;

                case Win32Interop.WM_MOUSEMOVE:
                    if (_down && !_dragging)
                    {
                        Win32Interop.GetCursorPos(out var p);
                        if (Math.Abs(p.X - _downPt.X) + Math.Abs(p.Y - _downPt.Y) > DragThresholdPx)
                            _dragging = true;
                    }
                    break;

                case Win32Interop.WM_LBUTTONUP:
                    bool wasDragging = _dragging;
                    _down = false;
                    _dragging = false;
                    if (PencilTriggerDecision.ShouldShowPencil(wasDragging))
                    {
                        Win32Interop.GetCursorPos(out var up);
                        ShowOverlay(up);
                    }
                    break;
            }
        }
        return Win32Interop.CallNextHookEx(_hook, nCode, wParam, lParam);
    }

    private bool CursorOverOverlay()
    {
        var o = _overlay;
        if (o == null || !o.Visible) return false;
        Win32Interop.GetCursorPos(out var p);
        var b = o.Bounds;
        return p.X >= b.Left && p.X <= b.Right && p.Y >= b.Top && p.Y <= b.Bottom;
    }

    // Defer UI off the hook callback to avoid re-entering the input path.
    private void ShowOverlay(Win32Interop.POINT at)
    {
        var o = _overlay;
        if (o == null) return;
        try { o.BeginInvoke(new Action(() => o.ShowAt(new System.Drawing.Point(at.X, at.Y)))); }
        catch { /* thread tearing down */ }
    }

    private void HideOverlay()
    {
        var o = _overlay;
        if (o == null) return;
        try { o.BeginInvoke(new Action(() => { if (o.Visible) o.Hide(); })); }
        catch { /* thread tearing down */ }
    }

    private void OnPencilClicked()
    {
        DRLogger.Log("Pencil: clicked", DRLogger.Category.APP);
        HideOverlay();
        try { _onTriggered(); }
        catch (Exception ex) { DRLogger.Error($"Pencil: onTriggered failed: {ex.Message}", DRLogger.Category.APP); }
    }
}
