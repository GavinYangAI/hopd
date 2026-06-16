package guiapp

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/GavinYangAI/hopd/internal/gui"
)

// showTestConnDialog renders a TestConnResult. When ssh recorded new host key(s)
// (accept-new already wrote ~/.ssh/known_hosts), it offers 信任并保存 / 取消; 取消
// removes the just-added entry via ssh-keygen -R.
func showTestConnDialog(win fyne.Window, hostName string, res gui.TestConnResult) {
	if len(res.Fingerprints) > 0 {
		showHostKeyTrust(win, hostName, res)
		return
	}
	if res.OK {
		dialog.ShowInformation("连接成功", "已成功连接到 "+hostName+"。", win)
		return
	}
	dialog.ShowError(errReason(res.Reason), win)
}

func showHostKeyTrust(win fyne.Window, hostName string, res gui.TestConnResult) {
	head := "首次连接 " + hostName + "，对方主机密钥如下："
	if !res.OK {
		head = "连接未成功（" + res.Reason + "），但已记录到主机密钥："
	}
	lines := fingerprintLines(res.Fingerprints)
	body := container.NewVBox(widget.NewLabel(head))
	for _, l := range lines {
		lbl := widget.NewLabel(l)
		lbl.TextStyle = mono
		body.Add(lbl)
	}
	body.Add(widget.NewLabel("信任并保存：保留到 ~/.ssh/known_hosts。\n取消：撤销刚才添加的记录。"))

	dlg := dialog.NewCustomConfirm("主机密钥", "信任并保存", "取消", body, func(trust bool) {
		if trust {
			return // accept-new already persisted the key
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func() {
			defer cancel()
			_ = gui.RemoveKnownHostEntry(ctx, hostName, gui.ExecRunner)
		}()
	}, win)
	dlg.Show()
}

// fingerprintLines formats each host key as "<host> <ALGO> <fingerprint>".
func fingerprintLines(keys []gui.HostKey) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		fp := k.Fingerprint
		if fp == "" {
			fp = "(未显示指纹)"
		}
		out = append(out, k.Host+"  "+k.Algo+"  "+fp)
	}
	return out
}

// errReason wraps a reason string as an error for dialog.ShowError.
func errReason(s string) error {
	if s == "" {
		s = "连接失败"
	}
	return reasonError(s)
}

type reasonError string

func (e reasonError) Error() string { return string(e) }
