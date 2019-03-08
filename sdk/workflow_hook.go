package sdk

// Those are icon for hooks
const (
	GitlabIcon    = "Gitlab"
	GitHubIcon    = "Github"
	BitbucketIcon = "Bitbucket"
)

// WorkflowHookModelBuiltin is a constant for the builtin hook models
const WorkflowHookModelBuiltin = "builtin"

//WorkflowNodeHookConfig represents the configguration for a WorkflowNodeHook
type WorkflowNodeHookConfig map[string]WorkflowNodeHookConfigValue

// GetBuiltinHookModelByName retrieve the hook model
func GetBuiltinHookModelByName(name string) *WorkflowHookModel {
	for _, m := range BuiltinHookModels {
		if m.Name == name {
			return m
		}
	}
	return nil
}

// GetBuiltinOutgoingHookModelByName retrieve the outgoing hook model
func GetBuiltinOutgoingHookModelByName(name string) *WorkflowHookModel {
	for _, m := range BuiltinOutgoingHookModels {
		if m.Name == name {
			return m
		}
	}
	return nil
}

//Values return values of the WorkflowNodeHookConfig
func (cfg WorkflowNodeHookConfig) Values(model WorkflowNodeHookConfig) map[string]string {
	r := make(map[string]string)
	for k, v := range cfg {
		if model[k].Configurable {
			r[k] = v.Value
		}
	}
	return r
}

// Clone returns a copied dinstance of cfg
func (cfg WorkflowNodeHookConfig) Clone() WorkflowNodeHookConfig {
	m := WorkflowNodeHookConfig(make(map[string]WorkflowNodeHookConfigValue, len(cfg)))
	for k, v := range cfg {
		m[k] = v
	}
	return m
}

// WorkflowNodeHookConfigValue represents the value of a node hook config
type WorkflowNodeHookConfigValue struct {
	Value        string `json:"value"`
	Configurable bool   `json:"configurable"`
	Type         string `json:"type"`
}

const (
	// HookConfigTypeString type string
	HookConfigTypeString = "string"
	// HookConfigTypeIntegration type integration
	HookConfigTypeIntegration = "integration"
	// HookConfigTypeProject type project
	HookConfigTypeProject = "project"
	// HookConfigTypeWorkflow type workflow
	HookConfigTypeWorkflow = "workflow"
	// HookConfigTypeHook type hook
	HookConfigTypeHook = "hook"
)

//WorkflowHookModel represents a hook which can be used in workflows.
type WorkflowHookModel struct {
	ID            int64                  `json:"id" db:"id" cli:"-"`
	Name          string                 `json:"name" db:"name" cli:"name"`
	Type          string                 `json:"type"  db:"type"`
	Author        string                 `json:"author" db:"author"`
	Description   string                 `json:"description" db:"description"`
	Identifier    string                 `json:"identifier" db:"identifier"`
	Icon          string                 `json:"icon" db:"icon"`
	Command       string                 `json:"command" db:"command"`
	DefaultConfig WorkflowNodeHookConfig `json:"default_config" db:"-"`
	Disabled      bool                   `json:"disabled" db:"disabled"`
}
