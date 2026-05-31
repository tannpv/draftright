import SwiftUI

/// Modal sheet shown for VietQR checkout.  Renders the QR image and
/// (when included) the manual-transfer fallback fields so users on
/// PCs / phones without a camera can still pay.  When [statusStream]
/// is provided, a live banner inside the sheet auto-dismisses on
/// confirmation.
struct QrCheckoutSheet: View {
    let imageURL: URL
    let bankInfo: BankInfo?
    let statusStream: AsyncStream<PaymentStatusUpdate>?
    var onClose: () -> Void

    var body: some View {
        VStack(alignment: .center, spacing: 14) {
            Text("Scan to pay")
                .font(.title3.weight(.bold))

            PaymentStatusBanner(stream: statusStream, onConfirmed: onClose)

            AsyncImage(url: imageURL) { phase in
                switch phase {
                case .success(let image):
                    image
                        .resizable()
                        .interpolation(.high)
                        .scaledToFit()
                        .frame(width: 260, height: 260)
                        .cornerRadius(12)
                case .failure:
                    Text("Could not load QR. Use manual transfer below.")
                        .multilineTextAlignment(.center)
                        .padding()
                        .frame(width: 260, height: 260)
                        .background(Color.gray.opacity(0.1))
                        .cornerRadius(12)
                default:
                    ProgressView()
                        .frame(width: 260, height: 260)
                }
            }

            Text("Open your banking app and scan this QR code.  Your plan activates automatically after payment.")
                .font(.caption)
                .foregroundColor(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 8)

            if let bank = bankInfo {
                Divider()
                Text("Or transfer manually")
                    .font(.subheadline.weight(.semibold))
                BankInfoTable(info: bank)
            }

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
