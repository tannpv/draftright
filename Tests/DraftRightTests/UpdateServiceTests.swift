import XCTest
import CryptoKit
@testable import DraftRight

/// Update manifest parsing, version comparison, and the integrity gate (#146).
///
/// The integrity check is the macOS half of #22 — it is what stops a corrupt or
/// tampered DMG being mounted — and it had no coverage at all until now.
///
/// Cases mirror DraftRightWindows.PureTests/UpdateManifestTests.cs so the
/// clients cannot drift on which release they decide to install.
@MainActor
final class UpdateServiceTests: XCTestCase {

    private func service(current: String = "2.3.30") -> UpdateService {
        UpdateService(currentVersion: current, backendUrl: "https://api.example.test")
    }

    private func decode(_ json: String) throws -> UpdateInfo {
        try JSONDecoder().decode(UpdateInfo.self, from: Data(json.utf8))
    }

    // MARK: - Version comparison

    func testIsNewerNumericAndPadding() {
        let s = service()
        let cases: [(String, String, Bool)] = [
            ("2.2.5",   "2.2.4",   true),
            ("2.2.4",   "2.2.4",   false),
            ("2.2.4",   "2.2.5",   false),
            ("2.10.0",  "2.9.0",   true),   // numeric, not lexical
            ("3.0.0",   "2.99.0",  true),
            ("2.2.0",   "2.2.0.0", false),  // missing component counts as 0
            ("2.2.0.1", "2.2.0",   true),
            ("",        "2.2.0",   false),  // garbage degrades to zeros
        ]
        for (remote, local, expected) in cases {
            XCTAssertEqual(s.isNewer(remote: remote, local: local), expected,
                           "isNewer(\(remote), \(local))")
        }
    }

    // MARK: - Manifest resolution
    //
    // The top-level `version` is a cross-platform maximum, so it can name a
    // release that mac_url does not point at. Preferring the per-platform entry
    // is what stops the "current 2.2.10, install 2.3.1, still 2.2.10" loop.

    func testResolvedPrefersThePlatformEntryOverTheLegacyEnvelope() throws {
        let info = try decode("""
        {"version":"2.3.31","mac_url":"https://x/DraftRight-2.3.30.dmg",
         "windows_url":"","linux_url":"","release_notes":"cross-platform",
         "required":false,
         "platforms":{"mac":{"version":"2.3.30","url":"https://x/mac-2.3.30.dmg",
                             "notes":"mac notes","required":false,"sha256":"AABB"},
                      "windows":{"version":"2.3.31","url":"https://x/win.exe",
                                 "notes":null,"required":false,"sha256":"CCDD"}}}
        """)

        let mac = info.resolved(for: "mac")
        XCTAssertEqual(mac.version, "2.3.30")
        XCTAssertEqual(mac.url, "https://x/mac-2.3.30.dmg")
        XCTAssertEqual(mac.notes, "mac notes")
        XCTAssertEqual(mac.sha256, "AABB")
    }

    func testResolvedFallsBackToLegacyFieldsWhenPlatformsAbsent() throws {
        let info = try decode("""
        {"version":"2.2.5","mac_url":"https://x/old.dmg","windows_url":"",
         "linux_url":"","release_notes":"legacy","required":true}
        """)

        let mac = info.resolved(for: "mac")
        XCTAssertEqual(mac.version, "2.2.5")
        XCTAssertEqual(mac.url, "https://x/old.dmg")
        XCTAssertEqual(mac.notes, "legacy")
        XCTAssertTrue(mac.required)
        // Legacy envelope carries no hash — the install proceeds unverified,
        // which is exactly the back-compat path #22 preserved.
        XCTAssertNil(mac.sha256)
    }

    func testResolvedInheritsEnvelopeNotesAndRequiredWhenTheEntryOmitsThem() throws {
        let info = try decode("""
        {"version":"2.3.0","mac_url":"https://x/a.dmg","windows_url":"",
         "linux_url":"","release_notes":"envelope notes","required":true,
         "platforms":{"mac":{"version":"2.3.0","url":"https://x/a.dmg",
                             "notes":null,"required":null,"sha256":null}}}
        """)

        let mac = info.resolved(for: "mac")
        XCTAssertEqual(mac.notes, "envelope notes")
        XCTAssertTrue(mac.required)
        XCTAssertNil(mac.sha256)
    }

    func testResolvedPicksTheRequestedPlatform() throws {
        let info = try decode("""
        {"version":"2.3.31","mac_url":"https://x/m.dmg","windows_url":"https://x/w.exe",
         "linux_url":"https://x/l.tar.gz","release_notes":"","required":false}
        """)
        XCTAssertEqual(info.resolved(for: "windows").url, "https://x/w.exe")
        XCTAssertEqual(info.resolved(for: "linux").url, "https://x/l.tar.gz")
        XCTAssertEqual(info.resolved(for: "mac").url, "https://x/m.dmg")
    }

    /// The shape production actually serves, so a backend field rename fails
    /// here rather than silently disabling updates.
    func testDecodesTheProductionManifestShape() throws {
        let info = try decode("""
        {"version":"2.3.32","mac_url":"https://draftright.info/downloads/a.dmg",
         "windows_url":"https://draftright.info/downloads/b.exe",
         "linux_url":"https://draftright.info/downloads/c.tar.gz",
         "mac_sha256":"ac3df6e6","windows_sha256":"bb062187",
         "release_notes":"- Fixed a thing","required":false,
         "platforms":{"mac":{"version":"2.3.30","url":"https://draftright.info/downloads/a.dmg",
                             "notes":"- Fixed a thing","required":false,"sha256":"ac3df6e6"}}}
        """)
        XCTAssertEqual(info.resolved(for: "mac").sha256, "ac3df6e6")
    }

    // MARK: - Integrity gate (#22)

    private func writeTemp(_ contents: String) throws -> String {
        let url = FileManager.default.temporaryDirectory
            .appendingPathComponent("dr-verify-\(UUID().uuidString).bin")
        try Data(contents.utf8).write(to: url)
        return url.path
    }

    private func sha256Hex(_ contents: String) -> String {
        SHA256.hash(data: Data(contents.utf8)).map { String(format: "%02x", $0) }.joined()
    }

    func testVerifyIntegrityAcceptsAMatchingHash() throws {
        let body = "pretend this is a DMG"
        let path = try writeTemp(body)
        defer { try? FileManager.default.removeItem(atPath: path) }

        XCTAssertNoThrow(try service().verifyIntegrity(at: path, expected: sha256Hex(body)))
    }

    func testVerifyIntegrityIsCaseAndWhitespaceInsensitive() throws {
        let body = "pretend this is a DMG"
        let path = try writeTemp(body)
        defer { try? FileManager.default.removeItem(atPath: path) }

        let padded = "  " + sha256Hex(body).uppercased() + "\n"
        XCTAssertNoThrow(try service().verifyIntegrity(at: path, expected: padded))
    }

    /// The case that matters: a hash that does not match must THROW, so the
    /// caller never mounts the image.
    func testVerifyIntegrityRejectsAMismatch() throws {
        let path = try writeTemp("the real payload")
        defer { try? FileManager.default.removeItem(atPath: path) }

        XCTAssertThrowsError(
            try service().verifyIntegrity(at: path, expected: sha256Hex("something else"))
        )
    }

    func testVerifyIntegrityRejectsTamperedContent() throws {
        let expected = sha256Hex("original bytes")
        // Same file path, different bytes — the tampering case.
        let path = try writeTemp("original bytes tampered")
        defer { try? FileManager.default.removeItem(atPath: path) }

        XCTAssertThrowsError(try service().verifyIntegrity(at: path, expected: expected))
    }

    /// Back-compat: releases published before hashes existed carry none, and
    /// must still install rather than hard-failing every older user.
    func testVerifyIntegrityPassesWhenNoHashWasPublished() throws {
        let path = try writeTemp("unverified but allowed")
        defer { try? FileManager.default.removeItem(atPath: path) }

        XCTAssertNoThrow(try service().verifyIntegrity(at: path, expected: nil))
        XCTAssertNoThrow(try service().verifyIntegrity(at: path, expected: ""))
        XCTAssertNoThrow(try service().verifyIntegrity(at: path, expected: "   "))
    }

    func testVerifyIntegrityThrowsWhenTheFileIsMissing() {
        let missing = FileManager.default.temporaryDirectory
            .appendingPathComponent("definitely-not-here.dmg").path
        XCTAssertThrowsError(
            try service().verifyIntegrity(at: missing, expected: String(repeating: "a", count: 64))
        )
    }
}
