import CoreGraphics

/// Shared keyboard layout metrics. Single source of truth so the keyboard
/// view, the toolbar, the accent popup, and the input-view height all stay
/// in sync (and theming is a one-place change).
public enum KeyboardDimensions {
    public static let rowHeight: CGFloat = 42
    public static let rowCount: CGFloat = 4
    public static let keyMargin: CGFloat = 3
    public static let keyRadius: CGFloat = 5

    public static let toolbarHeight: CGFloat = 44
    public static let diffSheetHeight: CGFloat = 280

    public static let accentCellWidth: CGFloat = 44
    public static let accentCellHeight: CGFloat = 48
    public static let accentCellGap: CGFloat = 4

    /// Total height of the QWERTY key area (all rows).
    public static var keyboardHeight: CGFloat { rowHeight * rowCount }
}
