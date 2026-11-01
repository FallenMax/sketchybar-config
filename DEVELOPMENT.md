# Development

改完必须在真机的 bar 上验证。下面的命令是本仓库实际的驱动方式，不开浏览器、不需要 GUI 自动化权限。

## Harness

`make verify` 是脚本化的证据，跑在真机的 bar 上，不需要人看截图。它先渲染一次（保证测的是当前代码产出的 bar），再断言：

- bar geometry：位置、margin、y_offset、高度、topmost 与摆放规则一致
- bar structure：item 列表与按当前空间推导的结构逐项一致
- pill insets：胶囊左右内边距对称（容差 2.5pt），用"bar 显示/隐藏"两帧的差分只取本工具画的像素
- no blank frame while rendering：渲染过程中连拍 8 帧，任一帧保留的本工具像素不得低于 60%
- invalid color falls back：非法颜色回落，`multicolor` 关键字透传

这三条都曾经真的坏过（左右不对称、每次渲染闪一下、结构重建），所以留在这里防回归。

```sh
make verify           # 只验证
make install          # 装到 ~/.config/sketchybar 并渲染一次
make doctor           # 依赖、服务、hook、signal、几何的只读体检
make reload           # 改 config.json 后重跑 sketchybarrc
```

驱动真实状态变化

```sh
yabai -m space --focus N                 # 空间切换
open -na TextEdit; killall TextEdit      # 窗口创建 / 销毁
yabai -m space --create                  # 空间变化（注意：不触发 space_created）
~/.config/sketchybar/update_sketchybar   # 手动渲染一次
sketchybar --query bar | space.<id> | space.<id>.<slot>
yabai -m query --spaces / --windows / --displays
cat ~/.cache/sketchybar-config/slots.json
```

写新断言时注意

- 视觉断言要能把"改坏"判红：改一个常量（比如 `endPadIcon`）重跑，必须红；只重构内部时必须绿。
- 胶囊是全圆角：要在边框行以内、两侧各跳过几列取墨迹，否则会把圆角或边框本身当成内容（`verify` 里 `pad`/`edge` 就是在做这件事）。
- 截图坐标是 pt 的 2 倍（Retina），用显示器宽度换算 scale。
- 隐藏 bar 会连带改变菜单栏自身的合成结果，边界不要从差分图推断，要用布局给的 rect。

清理

- `make verify` 自己用临时目录，无需清理。
- 临时改过的常量改回去并 `make install`；临时创建的 space 用 `yabai -m space <mission-control index> --destroy` 删掉。

## 已实测的环境事实

- SketchyBar 2.24：忙时 `--query bar` 会返回空或不完整的 JSON（`{}`、截断），必须校验 `position`/`height` 并重试。
- SketchyBar 布局：`icon.padding_*` 不影响 item 宽度；bracket 的 `background.padding_*` 不扩展绘制范围；`label.padding_*` 与 `label.width` 有效（空 label 也有效）。
- `--bar margin=N` 把 bar 窗口缩窄并居中（2560 宽 + margin 300 → 1960 宽、x=300）；`--bar position/y_offset/topmost/height` 可热改，不需要重启进程。
- 菜单栏高度与是否内建屏用一次 `swift -e`（`NSStatusBar.system.thickness` + `CGDisplayIsBuiltin`）取得，本机 22。
- yabai 7.1：`space_created` 对 `yabai -m space --create` 不触发；`mission_control_exit` 可用于 UI 路径。
- yabai 的窗口可见性：`is-visible` 对隐藏空间里的窗口一律为 false，所以它只有在"该空间当前可见"时才等价于"窗口真的在屏幕上"；`is-minimized` 只反映最小化，被应用自己藏起来的窗口可能两者都为 false。验证可见性用 `open -na Safari` → `yabai -m window <id> --minimize` → `yabai -m window --deminimize <id>` 观察 icon 增减（`verify` 的 window icons 一项会在每次运行时兜住）。
- `yabai -m config --space N bottom_padding` 可读可写，用于底部 bar 腾出条带；原值需自行记录还原。
- 日志：`$(brew --prefix)/var/log/sketchybar/sketchybar.{out,err}.log`；signal action 的输出去 `/tmp/yabai_$USER.out.log`，该文件被删除后 yabai 仍写旧 inode，要看新日志需重启 yabai。
