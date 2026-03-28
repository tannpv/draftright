import AppKit

@MainActor
final class ServiceProvider: NSObject {
    let appModel: AppModel
    init(appModel: AppModel) { self.appModel = appModel }
}
