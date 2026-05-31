import SwiftUI

/// Stacked list of payment-method rows.  One row per backend-enabled
/// method.  Tapping a row triggers [onSelect] which the parent wires
/// to `PaymentService.upgrade`.  Disabled while another method is
/// in flight so double-clicks don't spawn two checkouts.
struct MethodPickerView: View {
    let methods: [PaymentMethodKind]
    let isStarting: Bool
    let startingKind: PaymentMethodKind?
    let onSelect: (PaymentMethodKind) -> Void

    var body: some View {
        VStack(spacing: 8) {
            ForEach(methods, id: \.self) { kind in
                MethodRow(
                    descriptor: PaymentMethodDescriptor.forKind(kind),
                    isLoading: isStarting && startingKind == kind,
                    isDisabled: isStarting,
                    onTap: { onSelect(kind) }
                )
            }
        }
    }
}

private struct MethodRow: View {
    let descriptor: PaymentMethodDescriptor
    let isLoading: Bool
    let isDisabled: Bool
    let onTap: () -> Void

    var body: some View {
        Button(action: onTap) {
            HStack(spacing: 12) {
                Image(systemName: descriptor.symbolName)
                    .foregroundColor(.accentColor)
                    .frame(width: 24, alignment: .center)

                VStack(alignment: .leading, spacing: 2) {
                    Text(descriptor.displayName)
                        .font(.body.weight(.semibold))
                    Text(descriptor.description)
                        .font(.caption)
                        .foregroundColor(.secondary)
                }

                Spacer()

                if isLoading {
                    ProgressView()
                        .controlSize(.small)
                        .progressViewStyle(.circular)
                } else {
                    Image(systemName: "chevron.right")
                        .foregroundColor(.secondary)
                }
            }
            .padding(12)
            .background(
                RoundedRectangle(cornerRadius: 10)
                    .stroke(Color.secondary.opacity(0.2))
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .disabled(isDisabled)
    }
}
