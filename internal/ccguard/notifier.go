package ccguard

import (
	"fmt"
	"os/exec"
	"runtime"
)

// PlatformNotifier 跨平台通知器
type PlatformNotifier struct {
	bellEnabled  bool
	soundEnabled bool
}

// NewPlatformNotifier 创建平台通知器
func NewPlatformNotifier(bellEnabled, soundEnabled bool) *PlatformNotifier {
	return &PlatformNotifier{
		bellEnabled:  bellEnabled,
		soundEnabled: soundEnabled,
	}
}

// Bell 发送终端响铃
func (n *PlatformNotifier) Bell() {
	if n.bellEnabled {
		fmt.Print("\a")
	}
}

// Sound 播放系统通知声音
func (n *PlatformNotifier) Sound() {
	if !n.soundEnabled {
		return
	}

	go func() {
		var cmd *exec.Cmd

		switch runtime.GOOS {
		case "darwin":
			// macOS: 使用 afplay 播放系统声音
			cmd = exec.Command("afplay", "/System/Library/Sounds/Ping.aiff")
		case "linux":
			// Linux: 尝试 paplay (PulseAudio) 或 aplay (ALSA)
			if _, err := exec.LookPath("paplay"); err == nil {
				cmd = exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/complete.oga")
			} else if _, err := exec.LookPath("aplay"); err == nil {
				cmd = exec.Command("aplay", "-q", "/usr/share/sounds/alsa/Front_Center.wav")
			} else {
				// 回退到终端响铃
				fmt.Print("\a")
				return
			}
		case "windows":
			// Windows: 使用 PowerShell 播放系统声音
			cmd = exec.Command("powershell", "-c", "[System.Media.SystemSounds]::Beep.Play()")
		default:
			// 未知平台: 回退到终端响铃
			fmt.Print("\a")
			return
		}

		if cmd != nil {
			cmd.Run()
		}
	}()
}

// Notify 发送通知（同时触发响铃和声音）
func (n *PlatformNotifier) Notify() {
	n.Bell()
	n.Sound()
}
