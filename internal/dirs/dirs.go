package dirs

// StateDir is the root directory for all runbook runtime state files,
// relative to the project working directory.
const StateDir = "._runbook_state"

// ConfigDir is the directory where task configuration files are loaded from,
// relative to the project working directory.
const ConfigDir = ".runbook"

// OverridesFile is the path to the optional overrides file,
// relative to the project working directory.
const OverridesFile = ".runbook.overrides.yaml"
