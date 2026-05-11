// DraftRightMobile/ios/Shared/SharedKeychain.swift
import Foundation
import Security

/// A thin wrapper over the iOS Keychain that uses a shared access group so
/// the main app and the keyboard / action extensions all read and write
/// the same items.
///
/// **Target membership.** This file must be added to all three Xcode
/// targets — Runner, DraftRightKeyboard, and DraftRightAction — via the
/// File Inspector's "Target Membership" checkboxes. Do not duplicate the
/// file. Do not vendor it as a Swift Package; the targets are
/// app-extensions which can't depend on packages built outside the app
/// group.
///
/// **Access group.** `$(AppIdentifierPrefix)com.draftright.v2.shared`
/// must be present in each target's `*.entitlements` under
/// `keychain-access-groups`. The Apple Developer portal must also have
/// "Keychain Sharing" enabled with that access group on each App ID, and
/// fresh provisioning profiles regenerated. Without those steps, every
/// SecItemAdd / SecItemUpdate call below fails with errSecMissingEntitlement.
public enum SharedKeychain {
    public static let accessGroup = "com.draftright.v2.shared"
    private static let service = "com.draftright.v2"

    /// Set or update a value. Pass `nil` to delete the key.
    /// Returns `true` on success.
    @discardableResult
    public static func set(_ key: String, _ value: String?) -> Bool {
        guard let value = value else {
            return delete(key)
        }
        let data = Data(value.utf8)

        // Identity of the item: class + service + account + access group.
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
        ]

        // Try update first — covers the common case where the key already
        // exists from a prior write.
        let updateAttributes: [String: Any] = [
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleAfterFirstUnlock,
        ]
        let updateStatus = SecItemUpdate(query as CFDictionary, updateAttributes as CFDictionary)
        if updateStatus == errSecSuccess { return true }

        // Otherwise insert.
        var insert = query
        insert[kSecValueData as String] = data
        insert[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlock
        let addStatus = SecItemAdd(insert as CFDictionary, nil)
        return addStatus == errSecSuccess
    }

    public static func get(_ key: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne,
        ]
        var ref: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &ref)
        guard status == errSecSuccess, let data = ref as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }

    @discardableResult
    public static func delete(_ key: String) -> Bool {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecAttrAccessGroup as String: accessGroup,
        ]
        let status = SecItemDelete(query as CFDictionary)
        return status == errSecSuccess || status == errSecItemNotFound
    }
}
