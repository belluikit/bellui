//go:build darwin && (arm || arm64)

#import <os/log.h>

// log_line writes one line to the unified log. It uses os_log, not NSLog:
// NSLog writes to stderr instead whenever stderr is not /dev/null, and stderr
// may be the pipe this line came from.
void log_line(const char *line, int isErr) {
    os_log_with_type(OS_LOG_DEFAULT, isErr ? OS_LOG_TYPE_ERROR : OS_LOG_TYPE_DEFAULT, "Bellui: %{public}s", line);
}
