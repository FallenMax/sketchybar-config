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
- 手工送过 `--bar ...`（尤其 `margin`）之后必须跑一次 `~/.config/sketchybar/update_sketchybar`：bar 属性是**全局且持久**的，把 margin 设成 0 会让窗口盖住整条菜单栏、菜单全部点不动，而下一次渲染才会按胶囊重新收窄。`make doctor` 的「menu bar not covered」一项会兜住这种残留。

## 已实测的环境事实

- SketchyBar 2.24：忙时 `--query bar` 会返回空或不完整的 JSON（`{}`、截断），必须校验 `position`/`height` 并重试。
- SketchyBar 布局：`icon.padding_*` 不影响 item 宽度；bracket 的 `background.padding_*` 不扩展绘制范围；`label.padding_*` 与 `label.width` 有效（空 label 也有效）。
- `--bar margin=N` 把 bar 窗口缩窄并居中（2560 宽 + margin 300 → 1960 宽、x=300）；`--bar position/y_offset/topmost/height` 可热改，不需要重启进程。
- 菜单栏高度与是否内建屏用一次 `swift -e`（`NSStatusBar.system.thickness` + `CGDisplayIsBuiltin`）取得，本机 22。
- yabai 7.1：`space_created` 对 `yabai -m space --create` 不触发；`mission_control_exit` 可用于 UI 路径。
- yabai 的窗口状态实测（同一个窗口的四种状态）：

```
状态                        is-visible  is-minimized  is-hidden  has-ax-reference
当前空间、真的在屏幕上        true        false         false      true
其他空间                      false       false         false      true
最小化                        false       true          false      true
app 被隐藏（Cmd+H）           false       false         true       true
app 已关闭窗口但 yabai 还列着  false       false         false      false
```

  另外两种容易误判的窗口：`is-sticky=true` 的窗口会出现在**每一个**空间的 `windows` 列表里（只算一份，否则每个胶囊都重复）；`subrole=AXSystemDialog`（常见 `layer=unknown`）是 app 挂上去的面板/对话框，不是用户会切换的窗口，要排除。
  所以"窗口是否真的存在"看 `has-ax-reference`，"被藏起来"看 `is-minimized`/`is-hidden`，`is-visible` 只在当前空间可用。这也解释了一个坑：Spotify / 微信这类窗口在 yabai 里长期存在但 `has-ax-reference=false`，它们不是最小化，而是 app 已经不再持有该窗口。
- 验证可见性/最小化用 `open -na Safari` → `yabai -m window <id> --minimize` / `yabai -m window --deminimize <id>` 观察 icon 增减；隐藏 app 用 AX 写 `AXHidden`（`AXUIElementSetAttributeValue(app, "AXHidden", true)`，本机终端已有辅助功能权限）。`verify` 的 window icons 一项每次跑都会兜住这条规则。
- `yabai -m config --space N bottom_padding` 可读可写，用于底部 bar 腾出条带；原值需自行记录还原。
- SketchyBar 的 bar 窗口无法忽略鼠标：官方 bar 属性全集里没有该选项；issue #106 里作者明确拒绝增加 `clickthrough`（理由：macOS 不给"没画东西"的位置派发点击，要做只能靠全局 event tap + bounding rects，需要辅助功能权限）。那段讨论里的"透明即穿透"在本机对**菜单栏**不成立：bar 拉满宽时点 Apple 菜单/控制中心都无效，收窄窗口后同两个点立刻可用。私有 API `CGSSetWindowEventMask(cid, wid, 0)` 调用返回成功但行为不变。源码里 bar 窗口是 `SLSNewWindowWithOpaqueShapeAndContext(shape=整个 frame, opaque shape=空)`，没有按像素改事件掩码的逻辑。因此点击可用性靠收窄窗口实现，`--bar margin=N` 会把窗口缩到 `屏幕宽 - 2N` 并居中，且只改 margin 不会让胶囊闪（连拍 8 帧无空帧）。
- 验证点击是否可用：本机终端已有辅助功能权限，可以用 CGEvent 合成点击。注意 `CGEventCreateMouseEvent` 的坐标是 **point** 不是像素（Retina 屏要除以 2）；判据是点击前后截图的区域差异比例（菜单或面板展开会带来明显变化）。`apple menu(12,11)`、`file menu(225,11)`、`control center(2345,11)`、`clock(2480,11)` 是四个可直接复用的探针位置。
- 日志：`$(brew --prefix)/var/log/sketchybar/sketchybar.{out,err}.log`；signal action 的输出去 `/tmp/yabai_$USER.out.log`，该文件被删除后 yabai 仍写旧 inode，要看新日志需重启 yabai。
