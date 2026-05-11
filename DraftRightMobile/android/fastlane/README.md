fastlane documentation
----

# Installation

Make sure you have the latest version of the Xcode command line tools installed:

```sh
xcode-select --install
```

For _fastlane_ installation instructions, see [Installing _fastlane_](https://docs.fastlane.tools/#installing-fastlane)

# Available Actions

## Android

### android internal

```sh
[bundle exec] fastlane android internal
```

Upload AAB to Internal Testing track (smallest blast radius).

### android closed

```sh
[bundle exec] fastlane android closed
```

Upload AAB to Closed Testing (legacy name: 'alpha').

### android production_draft

```sh
[bundle exec] fastlane android production_draft
```

Upload AAB to Production track as a DRAFT (manual promote required).

### android production_rollout_10

```sh
[bundle exec] fastlane android production_rollout_10
```

Upload AAB to Production with a 10% staged rollout (use deliberately).

----

This README.md is auto-generated and will be re-generated every time [_fastlane_](https://fastlane.tools) is run.

More information about _fastlane_ can be found on [fastlane.tools](https://fastlane.tools).

The documentation of _fastlane_ can be found on [docs.fastlane.tools](https://docs.fastlane.tools).
