package internal

func ExecuteInHost(env []string, name string, args ...string) error {
	command := BuildCommand(env, name, args...)
	return command.Run()
}

func BashScriptExecuteInHost(line string) error {
	command := BuildCommand(nil, "bash", "-c", line)
	return command.Run()
}
