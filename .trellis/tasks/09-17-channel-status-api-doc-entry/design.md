## 设计

在 `components/user/monitor` 新增 `ChannelStatusApiDialog.vue`，由父页面控制 `show`。弹窗内展示固定且与 `docs/CHANNEL_STATUS_API.md` 一致的调用说明和 curl 示例；复制逻辑复用 `useClipboard`。V1 的 `MonitorHero` 与 V2 页面标题工具栏各增加一个入口按钮，共用同一弹窗组件。
