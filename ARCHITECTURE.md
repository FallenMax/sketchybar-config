# Architecture

## 作用力

- 这是 yabai 的插件，不是第二个窗口管理器：空间与窗口的真相由 yabai 拥有，bar 的呈现由 SketchyBar 拥有，渲染器自己不持有可演进的状态。
- 用户的成本是"装完就能用"：安装必须只动少量已知位置、可逆、失败能自己诊断。
- 对用户系统的侵入要低：除了底部 bar 需要的 `bottom_padding`（记录原值并还原），不写任何 yabai 配置。
- 余量：一次事件一个进程、一次渲染一次 sketchybar 调用，实测 ~80ms。重审条件：单屏空间数超过 ~12，或常态窗口数超过每空间 10。

## 设计

- `update_sketchybar`（Go 单包）无状态：每次从 yabai 读真相，重算结构，再一次性发 `--set`。
- 事件：18 个 yabai signal；yabai 重启会丢 signal，因此 `~/.yabairc` 里 source `yabai-hook.sh` 负责重注册并重绘。
- 持久状态只在 `~/.cache/sketchybar-config/`：`display.json`（内建屏 + 菜单栏高度）、`slots.json`（窗口→槽位、上次活跃空间、结构指纹、被替换的 bottom_padding）。
- 可见性：只给"真的在屏幕上"的窗口画 icon。yabai 对隐藏空间里的窗口一律报 `is-visible=false`，所以规则按空间区分——当前可见空间用 `is-visible`，其他空间用"未最小化且未隐藏"；这样既过滤掉当前空间里被系统藏起来的窗口，又不会把其他空间的胶囊清空。
- 结构：每个空间一组固定 item——`space.{id}.0`（空间号）、`.1..N`（窗口）、`.more`（+N）、`.end`（右侧留白）、`space.{id}`（bracket）、`space.{id}.gap`。只在结构指纹变化或 bar 为空时重建。
- 摆放（`layoutFor`）由主显示器宽度是否达到阈值（`wideDisplayWidth`）决定：够宽就顶部并设 margin 让开菜单栏两端控件；否则底部，并用 yabai `bottom_padding` 腾出条带。具体数值留在代码里，`verify` 会把实际值与期望值一起打印。
- 配置：`colors.border`、`colors.icon`（单色或 `multicolor`）、`maxWindowsPerSpace`、`apps[]`。
- 安装层：`make install` 只动 `~/.config/sketchybar/*`、`~/.yabairc` 两行、yabai signal、缓存目录；`make doctor` 逐项核对；`make uninstall` 全部还原。

## 决策

- 不常驻 daemon：事件驱动 + 一次性进程，省掉守护进程的生命周期与状态同步。
- 重建只看自记的结构指纹，不看 bar 的实时 item 列表：后者在空间切换时短暂失真，据此重建会让 bar 闪。
- 每次渲染只发 `--set`；`--bar` / `--default` 只在重建时发——`--bar` 会触发整条 bar 重排，表现为胶囊消失一瞬。
- 横向留白只能来自 `icon.width`、`label.padding_*`、`label.width`：SketchyBar 忽略 icon padding，bracket 的 background padding 也不扩展绘制范围。右侧留白因此由尾部占位项 `.end` 承担，其宽度按"空槽位与空 `.more` 各占 1pt"逐次补偿，使尾部留白与窗口数无关。
- 尾部留白按 item 盒子计算，所以最后一枚图标的墨迹到边框的距离仍会随字形宽度浮动几个 pt；这是字形本身的性质，不再为它调参。
- 颜色拆成 border / icon 两个开关，默认都是蓝色；非活跃空间统一降 alpha(0x88)，不引入第三套颜色。
- 不使用 SketchyBar 的 space 事件驱动渲染：一次切换会连发多次 `space_windows_change`，渲染叠加反而更闪。
- 摆放按显示器宽度而不是"是否内建屏"判定：够宽的屏顶部更省空间，与刘海无关。
- 被否决：按内容宽度动态设 margin（每次开关窗口都 relayout，把闪烁带回来）；改 SketchyBar 源码加"忽略鼠标"；常驻 daemon。

## 偏差

- 宽屏下 bar 窗口仍是半屏宽的中央条带，胶囊正上方的菜单栏空白区仍会吃掉点击；彻底消除需要动态 margin 或改 SketchyBar。
- `space_created` 对 `yabai -m space --create` 不触发（实测 action 未执行）：CLI 建空间后要等下一个事件或 `make reload`；UI 建空间由 `mission_control_exit` 兜住。
- `~/.yabairc` 注入是唯一一处改用户配置的写操作；用户若把 yabai 配置放在别处，重启 yabai 后 signal 不会重注册（`make doctor` 能查出）。
- 关键不变量（摆放几何、item 结构、左右内边距对称、渲染不闪、非法颜色回落）由 `make verify` 守住；其余靠 `make doctor` 的只读体检。
