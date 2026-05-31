import SwiftUI
import AppKit

/// Modal sheet shown for `bank_transfer` checkout.  Renders the
/// account fields plus a copyable reference code.  Auto-dismisses
/// when the foreground poller reports success (server-side webhook
/// activated the subscription).
struct BankTransferSheet: View {
    let info: BankInfo
    let statusStream: AsyncStream<PaymentStatusUpdate>?
    var onClose: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Bank transfer")
                .font(.title3.weight(.bold))

            PaymentStatusBanner(stream: statusStream, onConfirmed: onClose)

            Text("Transfer this exact amount from any Vietnamese bank.  The reference code links the payment to your account; your plan activates automatically once received.")
                .font(.caption)
                .foregroundColor(.secondary)
                .fixedSize(horizontal: false, vertical: true)

            BankInfoTable(info: info)

            HStack {
                Spacer()
                Button("Close", action: onClose)
                    .keyboardShortcut(.cancelAction)
            }
            .padding(.top, 4)
        }
        .padding(20)
        .frame(width: 420)
    }
}

/// Shared field-grid used by both [QrCheckoutSheet] and
/// [BankTransferSheet].  Each row has a label + value; copyable rows
/// get a tiny copy button that writes to the pasteboard.
struct BankInfoTable: View {
    let info: BankInfo

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            row(label: "Bank",      value: info.bankName,      copyable: false)
            row(label: "Account #", value: info.accountNumber, copyable: true)
            row(label: "Name",      value: info.accountName,   copyable: false)
            row(label: "Amount",
                value: "\(amountString(info.amount)) \(info.currency)",
                copyable: true)
            row(label: "Reference", value: info.reference, copyable: true,
                hint: "Must include this in the transfer description.")
        }
    }

    @ViewBuilder
    private func row(label: String, value: String, copyable: Bool, hint: String? = nil) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Text(label)
                .font(.caption)
                .foregroundColor(.secondary)
                .frame(width: 84, alignment: .leading)
            Text(value)
                .font(.system(.body, design: .monospaced))
                .textSelection(.enabled)
            Spacer()
            if copyable {
                Button {
                    let pb = NSPasteboard.general
                    pb.clearContents()
                    pb.setString(value, forType: .string)
                } label: {
                    Image(systemName: "doc.on.doc")
                }
                .buttonStyle(.borderless)
                .help("Copy \(label)")
            }
        }
        if let hint {
            Text(hint)
                .font(.caption2)
                .foregroundColor(.secondary)
                .padding(.leading, 92)
        }
    }

    private func amountString(_ value: Double) -> String {
        if value == value.rounded() {
            return String(Int(value))
        }
        return String(format: "%.2f", value)
    }
}
