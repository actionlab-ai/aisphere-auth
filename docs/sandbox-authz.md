# Sandbox 相关授权

新增资源对象建议：

- `aihub:sandbox-profile:<profileId>`: `read/use/manage/publish`
- `aihub:sandbox:<sandboxId>`: `read/run/delete`
- `aihub:sandbox-policy:<targetId>`: `read/manage`

`aisphere-sandbox` Adapter 调用 `aisphere-auth` 时，只校验平台调用身份和 profile 使用权限；Agent/Skill/Tool 的授权仍由 Hub resolve 阶段完成。
