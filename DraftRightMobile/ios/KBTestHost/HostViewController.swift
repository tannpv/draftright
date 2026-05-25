import UIKit

/// Minimal host for keyboard UI tests. Two text fields the XCUITest harness
/// can locate by accessibility id, focus, and read back. Field 2 lets a test
/// switch the editing context (without dismissing the keyboard) to verify the
/// Telex composer doesn't leak a stale composition into the next field.
/// Flutter's Runner renders to an opaque GL surface that XCUITest cannot
/// introspect, so the keyboard extension is exercised against these native
/// fields instead.
final class HostViewController: UIViewController {

    static let fieldIdentifier = "kb_target_field"
    static let field2Identifier = "kb_target_field_2"

    private let field = UITextField()
    private let field2 = UITextField()

    private func configure(_ tf: UITextField, id: String) {
        tf.borderStyle = .roundedRect
        tf.accessibilityIdentifier = id
        tf.autocorrectionType = .no
        tf.autocapitalizationType = .none
        tf.smartDashesType = .no
        tf.smartQuotesType = .no
        tf.spellCheckingType = .no
        tf.translatesAutoresizingMaskIntoConstraints = false
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground

        configure(field, id: Self.fieldIdentifier)
        configure(field2, id: Self.field2Identifier)
        view.addSubview(field)
        view.addSubview(field2)

        NSLayoutConstraint.activate([
            field.leadingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.leadingAnchor, constant: 20),
            field.trailingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.trailingAnchor, constant: -20),
            field.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 40),
            field.heightAnchor.constraint(equalToConstant: 44),

            field2.leadingAnchor.constraint(equalTo: field.leadingAnchor),
            field2.trailingAnchor.constraint(equalTo: field.trailingAnchor),
            field2.topAnchor.constraint(equalTo: field.bottomAnchor, constant: 16),
            field2.heightAnchor.constraint(equalToConstant: 44),
        ])
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        field.becomeFirstResponder()
    }
}
