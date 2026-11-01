# Development

改完必须在真机的 bar 上验证。下面的命令是本仓库实际的驱动方式，不开浏览器、不需要 GUI 自动化权限。

## Harness

启动

```sh
cd ~/code/github/sketchybar-config
make install          # 构建、装到 ~/.config/sketchybar、注册 signal、重启服务、渲染一次
```

可用标志：`make install` 结尾打印 `rendered: N spaces on the main display, ...`，且屏幕上出现胶囊。进程活着不算。

自检

```sh
make doctor           # 依赖、scripting addition、服务、字体、config、hook、signal、bar 几何、item 数
```

全部 ✓ 才算实例可用。

驱动

```sh
make reload                              # 重跑 sketchybarrc（改 config.json 后用）
yabai -m space --focus N                 # 制造空间切换
open -na TextEdit; killall TextEdit      # 制造窗口创建/销毁
yabai -m space --create                  # 制造空间变化（注意：不触发 space_created）
~/.config/sketchybar/update_sketchybar   # 手动渲染一次
```

读回状态

```sh
sketchybar --query bar | python3 -m json.tool            # position/margin/y_offset/height/items
sketchybar --query space.<id> | python3 -m json.tool     # bracket 背景、边框、bounding_rects
sketchybar --query space.<id>.<slot>                     # 单个 item 的 icon/label/padding
yabai -m query --spaces / --windows / --displays
cat ~/.cache/sketchybar-config/slots.json
```

证据

- 视觉：`screencapture -x /tmp/x.png`，再用 python + PIL 只取中线行或指定 x 范围统计像素。判断"有没有闪""间距对不对"必须走像素统计，肉眼看截图会漏掉 1~2 帧事件。
- 几何：`bounding_rects` 给 pt 单位的真实布局；胶囊是全圆角，边框的水平/垂直极值只在中线出现，量墨迹内边距只能取中线附近几行，否则会把圆角当内容。
- 像素坐标是 pt 的 2 倍（Retina）。

清理

- 自己开的临时应用用完 `killall`；临时改过的常量（阈值、offset）改回去并重新 `make install`。
- 为了验证而临时创建的 space 用 `yabai -m space <mission-control index> --destroy` 删掉。
- 证据留在 `/tmp`，不要清。

## 已实测的环境事实

- SketchyBar 2.24：忙时 `--query bar` 会返回空或不完整的 JSON（`{}`、截断），必须校验 `position`/`height` 并重试。
- SketchyBar 布局：`icon.padding_*` 不影响 item 宽度；bracket 的 `background.padding_*` 不扩展绘制范围；`label.padding_*` 与 `label.width` 有效（空 label 也有效）。
- `--bar margin=N` 把 bar 窗口缩窄并居中（2560 宽 + margin 300 → 1960 宽、x=300）；`--bar position/y_offset/topmost/height` 可热改，不需要重启进程。
- 菜单栏高度与是否内建屏用一次 `swift -e`（`NSStatusBar.system.thickness` + `CGDisplayIsBuiltin`）取得，本机 22。
- yabai 7.1：`space_created` 对 `yabai -m space --create` 不触发；`mission_control_exit` 可用于 UI 路径。
- `yabai -m config --space N bottom_padding` 可读可写，用于底部 bar 腾出条带；原值需自行记录还原。
- 日志：`$(brew --prefix)/var/log/sketchybar/sketchybar.{out,err}.log`；signal action 的输出去 `/tmp/yabai_$USER.out.log`，该文件被删除后 yabai 仍写旧 inode，要看新日志需重启 yabai。
