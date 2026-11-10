package markstream

import "strings"

// Labels overrides built-in control text. Keys are the default English labels.
// Native editor menus additionally follow MyGo's platform locale.
type Labels map[string]string

func tr(labels Labels, key string) string {
	if text := labels[key]; text != "" {
		return text
	}
	return key
}
func labelsFor(locale string, custom Labels) Labels {
	result := Labels{}
	if strings.HasPrefix(strings.ToLower(locale), "zh") {
		for key, text := range map[string]string{"Copy": "复制", "Copied": "已复制", "Copy Markdown": "复制 Markdown", "Copy file": "复制文件", "Copy diff": "复制差异", "Collapse": "收起", "Expand": "展开", "Code collapsed": "代码已收起", "Receiving": "接收中", "Changes": "变更", "Split": "并排", "Unified": "统一", "Hide unchanged": "隐藏未变更内容", "Show unchanged": "显示未变更内容", "Source": "源码", "Preview": "预览", "Fit": "适应窗口", "Fullscreen": "全屏", "Close preview": "关闭预览", "Close image": "关闭图片", "Rendering…": "渲染中…", "Retry": "重试", "Image": "图片"} {
			result[key] = text
		}
	}
	for key, text := range custom {
		result[key] = text
	}
	return result
}
func (v *viewState) label(key string) string { return tr(v.props.Labels, key) }
