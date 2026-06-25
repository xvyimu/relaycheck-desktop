import { type ReactNode } from "react";
import { cn } from "@/lib/cn";

export type TwoFactorGuideVariant = "inline" | "dialog";

export interface TwoFactorGuideProps {
  /** 上游站点名称，用于在指引中指代目标站点。 */
  siteName?: string;
  /** 上游站点根地址，例如 https://newapi.example.com。 */
  baseUrl?: string;
  /** 登录页地址，未提供时会基于 baseUrl 推断 /login。 */
  loginUrl?: string;
  /** 渲染形态：inline 嵌入账号卡/详情；dialog 作为弹层遮罩。 */
  variant?: TwoFactorGuideVariant;
  /** 是否默认展开步骤详情，inline 默认展开，dialog 默认展开。 */
  defaultExpanded?: boolean;
  /** 关闭回调，dialog 形态下点击遮罩或关闭按钮触发。 */
  onClose?: () => void;
  /** 点击“打开网页登录”按钮时的回调，便于直接触发浏览器登录流程。 */
  onOpenBrowserLogin?: () => void;
  /** 附加 className。 */
  className?: string;
  /** 顶部标题，默认“二次验证（2FA）登录指引”。 */
  title?: string;
  /** 自定义底部补充说明。 */
  footer?: ReactNode;
}

/**
 * TwoFactorGuide 展示 2FA 登录的明确操作指引。
 *
 * 当用户尝试登录支持 2FA 的中转站点（NewAPI / OneAPI / Sub2API 等）时，
 * 密码登录通常会被站点拒绝并要求二次验证。本组件提供分步说明，引导用户
 * 通过网页登录完成 2FA 验证并保存授权会话。
 */
export function TwoFactorGuide({
  siteName,
  baseUrl,
  loginUrl,
  variant = "inline",
  defaultExpanded = true,
  onClose,
  onOpenBrowserLogin,
  className,
  title = "二次验证（2FA）登录指引",
  footer,
}: TwoFactorGuideProps) {
  const resolvedLoginUrl = loginUrl || (baseUrl ? `${baseUrl.replace(/\/+$/, "")}/login` : "");
  const siteLabel = siteName || baseUrl || "该站点";

  const steps: Array<{ title: string; description: string }> = [
    {
      title: "点击“网页登录”打开浏览器",
      description: `RelayCheck 会用独立浏览器配置打开 ${siteLabel} 的登录页，不会影响你日常浏览器的登录态。`,
    },
    {
      title: "输入账号密码并提交",
      description: "在打开的浏览器窗口中正常填写用户名/邮箱与密码，先完成基础登录。",
    },
    {
      title: "完成二次验证",
      description:
        "站点会要求输入 2FA 验证码。常见形式：TOTP 动态码（Google Authenticator / 1Password 等扫码绑定的 6 位数字）、短信/邮箱验证码、或备用恢复码。请在对应渠道获取并填入。",
    },
    {
      title: "确认登录成功后回到 RelayCheck",
      description: "浏览器显示登录成功（进入控制台或首页）后，回到本页面点击“保存授权”按钮，RelayCheck 会读取并加密保存当前会话。",
    },
    {
      title: "重新执行签到 / 余额刷新",
      description: "授权保存成功后，登录态会变为“登录有效”，即可正常签到与刷新余额。",
    },
  ];

  const tips: string[] = [
    "密码登录接口无法绕过 2FA，必须通过网页登录完成验证。",
    "TOTP 验证码每 30 秒刷新一次，若提示过期请等待下一周期再填。",
    "若丢失 2FA 设备，可使用站点提供的备用恢复码，或联系站点管理员重置。",
    "保存授权后，会话有效期取决于站点配置；过期后需重新走网页登录流程。",
  ];

  const content = (
    <div className="twofa-guide-body">
      <p className="twofa-guide-lead">
        <strong>{siteLabel}</strong> 启用了二次验证（2FA）。密码登录会被站点拦截，请按以下步骤通过网页登录完成验证。
        {resolvedLoginUrl ? (
          <>
            {" "}登录页：<code>{resolvedLoginUrl}</code>
          </>
        ) : null}
      </p>

      <ol className="twofa-guide-steps">
        {steps.map((step, index) => (
          <li className="twofa-guide-step" key={step.title}>
            <span className="twofa-step-index">{index + 1}</span>
            <div className="twofa-step-text">
              <strong>{step.title}</strong>
              <span>{step.description}</span>
            </div>
          </li>
        ))}
      </ol>

      <details className="twofa-guide-tips" open={defaultExpanded}>
        <summary>2FA 常见问题与提示</summary>
        <ul>
          {tips.map((tip) => (
            <li key={tip}>{tip}</li>
          ))}
        </ul>
      </details>

      <div className="twofa-guide-actions">
        {onOpenBrowserLogin ? (
          <button type="button" className="twofa-guide-primary" onClick={onOpenBrowserLogin}>
            打开网页登录
          </button>
        ) : null}
        {onClose ? (
          <button type="button" className="ghost" onClick={onClose}>
            {variant === "dialog" ? "关闭" : "我知道了"}
          </button>
        ) : null}
      </div>

      {footer ? <div className="twofa-guide-footer">{footer}</div> : null}
    </div>
  );

  if (variant === "dialog") {
    return (
      <div
        className="twofa-guide-backdrop"
        role="presentation"
        onMouseDown={(event) => {
          if (event.target === event.currentTarget) {
            onClose?.();
          }
        }}
      >
        <aside
          aria-label={title}
          aria-modal="true"
          className={cn("twofa-guide twofa-guide-dialog", className)}
          role="dialog"
          tabIndex={-1}
        >
          <header className="twofa-guide-head">
            <h3>{title}</h3>
            {onClose ? (
              <button type="button" className="ghost twofa-guide-close" aria-label="关闭" onClick={onClose}>
                ×
              </button>
            ) : null}
          </header>
          {content}
        </aside>
      </div>
    );
  }

  return (
    <section className={cn("twofa-guide twofa-guide-inline", className)} role="note" aria-label={title}>
      <header className="twofa-guide-head">
        <h3>{title}</h3>
      </header>
      {content}
    </section>
  );
}
