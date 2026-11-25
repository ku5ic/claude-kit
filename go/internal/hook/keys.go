package hook

// The payload keys checks read, as Payload's dotted paths. A mistyped
// literal reads "" and the check fails open, so reads go through these;
// TestPayloadKeys checks each against the hooks docs' example inputs.
const (
	KeySessionID            = "session_id"
	KeyTranscriptPath       = "transcript_path"
	KeyCwd                  = "cwd"
	KeyPermissionMode       = "permission_mode"
	KeyHookEventName        = "hook_event_name"
	KeySource               = "source"
	KeyPrompt               = "prompt"
	KeyExpansionType        = "expansion_type"
	KeyCommandName          = "command_name"
	KeyStopHookActive       = "stop_hook_active"
	KeyLastAssistantMessage = "last_assistant_message"
	KeyAgentID              = "agent_id"
	KeyAgentType            = "agent_type"
	KeyAgentTranscriptPath  = "agent_transcript_path"
	KeyToolName             = "tool_name"
	KeyToolInput            = "tool_input"
	KeyToolUseID            = "tool_use_id"

	KeyInputCommand      = "tool_input.command"
	KeyInputFilePath     = "tool_input.file_path"
	KeyInputContent      = "tool_input.content"
	KeyInputOldString    = "tool_input.old_string"
	KeyInputNewString    = "tool_input.new_string"
	KeyInputReplaceAll   = "tool_input.replace_all"
	KeyInputEdits        = "tool_input.edits"
	KeyInputNotebookPath = "tool_input.notebook_path"
	KeyInputPath         = "tool_input.path"
	KeyInputGlob         = "tool_input.glob"
	KeyInputPlan         = "tool_input.plan"
	KeyInputPlanFilePath = "tool_input.planFilePath"
	KeyInputSkill        = "tool_input.skill"
	KeyInputSubagentType = "tool_input.subagent_type"
	KeyInputMessage      = "tool_input.message"
	// KeyInputBefore is the kit's own: bash-edits' synthetic payload carries
	// the file as it was before the command.
	KeyInputBefore = "tool_input.before"
)
