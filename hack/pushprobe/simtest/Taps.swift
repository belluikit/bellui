import XCTest

// Drives SpringBoard for hack/pushprobe: the permission alert and banners.
// The text to find comes from the environment (TAP_TEXT), the wait from TAP_WAIT.
final class Taps: XCTestCase {
    let sb = XCUIApplication(bundleIdentifier: "com.apple.springboard")

    func env(_ k: String, _ d: String) -> String { ProcessInfo.processInfo.environment[k] ?? d }

    func testAllow() { tapAlert("Allow") }
    func testDontAllow() { tapAlert("Don’t Allow") }

    func tapAlert(_ label: String) {
        let b = sb.alerts.buttons[label]
        XCTAssertTrue(b.waitForExistence(timeout: 20), "no \(label) button")
        b.tap()
    }

    // Taps the banner whose text contains TAP_TEXT, waiting TAP_WAIT seconds for it.
    func testTapBanner() {
        let text = env("TAP_TEXT", "pushprobe")
        let wait = Double(env("TAP_WAIT", "40")) ?? 40
        let p = NSPredicate(format: "label CONTAINS %@", text)
        let el = sb.descendants(matching: .any).matching(p).firstMatch
        XCTAssertTrue(el.waitForExistence(timeout: wait), "no banner with \(text)")
        el.tap()
        sleep(3)
    }

    // Long-presses the banner with TAP_TEXT and taps its TAP_ACTION button.
    func testTapAction() {
        let text = env("TAP_TEXT", "pushprobe")
        let action = env("TAP_ACTION", "Reply")
        let p = NSPredicate(format: "label CONTAINS %@", text)
        let el = sb.descendants(matching: .any).matching(p).firstMatch
        XCTAssertTrue(el.waitForExistence(timeout: 30), "no banner with \(text)")
        el.press(forDuration: 1.5)
        let b = sb.buttons[action]
        XCTAssertTrue(b.waitForExistence(timeout: 10), "no \(action) button")
        b.tap()
        sleep(3)
    }
}
