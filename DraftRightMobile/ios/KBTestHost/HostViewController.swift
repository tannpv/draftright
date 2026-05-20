import UIKit

/// Minimal host for keyboard UI tests. A single text field that the
/// XCUITest harness can locate by accessibility id, focus, and read back.
/// Flutter's Runner renders to an opaque GL surface that XCUITest cannot
/// introspect, so the keyboard extension is exercised against this native
/// field instead.
final class HostViewController: UIViewController {

    static let fieldIdentifier = "kb_target_field"

    private let field = UITextField()

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground

        field.borderStyle = .roundedRect
        field.accessibilityIdentifier = Self.fieldIdentifier
        field.autocorrectionType = .no
        field.autocapitalizationType = .none
        field.smartDashesType = .no
        field.smartQuotesType = .no
        field.spellCheckingType = .no
        field.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(field)

        NSLayoutConstraint.activate([
            field.leadingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.leadingAnchor, constant: 20),
            field.trailingAnchor.constraint(equalTo: view.safeAreaLayoutGuide.trailingAnchor, constant: -20),
            field.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor, constant: 40),
            field.heightAnchor.constraint(equalToConstant: 44),
        ])
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        field.becomeFirstResponder()
    }
}
