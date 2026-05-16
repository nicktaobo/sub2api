package service

// defaultTermsContentEN 与 defaultTermsContentZhTW 是平台首次启用登录条款时使用的默认文案。
// 仅在管理员尚未保存自定义条款时返回；已配置的条款不会被覆盖。
//
// 改动须知：修改文案后请同步更新 login_agreement_updated_at 以触发用户重新同意。

const defaultTermsContentEN = `Welcome to the AI Token infrastructure and API routing services provided by this platform.

Before registering, logging in, creating an API Key, or using any services, please carefully read these Terms of Service. By accessing or using the platform, you acknowledge that you have read, understood, and agreed to all terms herein.

## 1. Nature of Service

This platform is an international AI API routing and compute resource platform providing:
* AI Token compute resources;
* API routing and forwarding;
* Model load balancing and scheduling;
* Developer-oriented technical support services.

The platform does not directly train, fine-tune, own, or generate any AI models. All model capabilities are provided by third-party AI model providers.

## 2. Service Availability & Restricted Regions

The platform does not currently provide services to users located within the People's Republic of China mainland.

Users are responsible for ensuring that their location, network environment, payment methods, and use of the services comply with all applicable laws, regulations, sanctions, and international compliance requirements within their jurisdiction.

The platform reserves the right to reject service, suspend access, or terminate accounts associated with restricted regions, sanctioned entities, high-risk users, or compliance concerns.

## 3. User Responsibilities & Prohibited Conduct

Users must use the platform lawfully and in compliance with applicable regulations.

Users may not use the platform for activities including but not limited to:
* Generating unlawful, violent, fraudulent, hateful, or explicit content;
* Cyberattacks, vulnerability exploitation, scraping abuse, or malicious resource consumption;
* Fraud, money laundering, illegal payment activities, or unlawful commercial operations;
* Violating third-party intellectual property rights, privacy rights, or data rights;
* Circumventing platform security, rate limits, billing systems, or risk controls;
* Spam distribution, fake customer support, or abusive automated operations;
* Violating policies imposed by upstream AI model providers.

The platform reserves the right to restrict, suspend, or terminate services for abusive, suspicious, or non-compliant activities.

## 4. Data, Privacy & Logging Policy

The platform values user privacy and data security.

Except for technical information necessary for service operation, security protection, billing, auditing, and legal compliance, the platform generally does not proactively retain users' prompts, uploaded files, model outputs, or business data on a long-term basis.

User data will not be used for:
* AI model training;
* Model fine-tuning;
* Advertising or marketing purposes;
* Commercial resale or data brokerage.

However, the platform may conduct necessary logging, auditing, and security analysis in situations involving:
* Security incidents;
* Abuse detection;
* Abnormal traffic or resource usage;
* Legal, regulatory, or judicial requirements.

Certain requests may be processed by third-party AI providers, and related data handling practices may also be subject to the policies of those providers.

## 5. Third-Party Models & Service Providers

Certain platform capabilities rely on third-party AI model providers and cloud vendors, including but not limited to:
* OpenAI
* Anthropic
* Google
* xAI
* DeepSeek

Users acknowledge and agree that:
* Model outputs, stability, availability, retention behavior, and policy restrictions are not fully controlled by the platform;
* Upstream providers may change pricing, quotas, capabilities, or policies at any time;
* The platform shall not be directly liable for interruptions, restrictions, policy changes, outages, or force majeure events affecting third-party services.

## 6. AI-Generated Content Disclaimer

AI-generated content is probabilistic in nature and may contain inaccuracies, hallucinations, or unintended outputs.

The platform makes no representations or warranties regarding:
* Accuracy;
* Authenticity;
* Legality;
* Completeness;
* Reliability;
* Fitness for a particular purpose;
* Non-infringement.

Users are solely responsible for evaluating and using AI-generated content and assume all associated risks and liabilities.

Where required by law or regulation, the platform may apply content labeling, safety controls, or output restrictions.

## 7. Account & API Key Security

Users are responsible for safeguarding their accounts, API Keys, access tokens, and related credentials.

Any losses or liabilities arising from:
* Credential leakage;
* Unauthorized access;
* Abuse of API Keys;
* Account compromise;
* Unauthorized usage charges;

shall be borne solely by the user.

The platform reserves the right to suspend or restrict suspicious traffic, high-risk requests, abnormal API activity, or potential attacks.

## 8. Billing, Credits & Refunds

Services are billed according to the pricing and usage policies published by the platform.

Token consumption, API usage counts, bandwidth usage, and related statistics shall be determined based on the platform's system records.

Unless otherwise required by applicable law:
* Consumed resources are generally non-refundable;
* The platform may, at its discretion, provide adjustments or compensation for failed requests caused by upstream outages or technical abnormalities;
* Pricing, quotas, plans, and billing structures may be updated at any time.

## 9. Service Availability

The platform will make commercially reasonable efforts to maintain stable and highly available services but does not guarantee uninterrupted availability.

The platform reserves the right to upgrade, limit, suspend, or discontinue any models, APIs, or features due to:
* System maintenance;
* Capacity adjustments;
* Security and risk control;
* Upstream provider changes;
* Legal or regulatory requirements.

## 10. Modification of Terms

The platform reserves the right to update or modify these Terms at any time based on business, legal, regulatory, or compliance requirements.

Updated Terms become effective upon publication on the platform. Continued use of the services constitutes acceptance of the revised Terms.

## 11. Suspension & Termination

The platform reserves the right to suspend or terminate services if users:
* Violate these Terms;
* Engage in unlawful or high-risk activities;
* Abuse platform resources;
* Threaten platform security or operational stability;
* Violate upstream provider policies.

The platform further reserves all rights to pursue legal remedies where applicable.

## 12. Miscellaneous

These Terms shall be interpreted in accordance with applicable legal principles and international commercial practices.

If any provision of these Terms is deemed invalid or unenforceable, the remaining provisions shall remain in full force and effect.
`

const defaultTermsContentZhTW = `歡迎使用本平台提供之 AI Token 算力與 API 轉接服務。

使用者於註冊、登入、建立 API Key 或使用本平台服務前，請務必詳閱本協議。使用本服務即視為已閱讀、理解並同意本協議全部內容。

## 1. 服務性質

本平台為國際化 AI API 路由與算力資源平台，主要提供：
* AI Token 算力資源；
* API 介面轉接；
* 模型路由與負載調度；
* 開發者技術支援服務。

本平台不直接訓練、微調、生產或擁有第三方 AI 模型，相關模型能力均來自第三方模型服務供應商。

## 2. 服務對象與地區限制

本平台目前不面向中華人民共和國境內用戶提供服務。

使用者應自行確認其所在地、網路環境、付款方式及使用行為符合所在地法律法規及相關國際合規要求。

若平台判定使用者屬受限制地區、受制裁對象、高風險主體或存在合規風險，平台有權拒絕服務、限制功能或終止帳戶。

## 3. 使用者責任與禁止行為

使用者應合法、合規使用本平台服務，不得利用平台從事包括但不限於：
* 違法、暴力、色情、仇恨、詐騙等內容生成；
* 網路攻擊、漏洞利用、爬蟲濫用或惡意消耗資源；
* 洗錢、非法支付或其他違法商業活動；
* 侵害第三方智慧財產權、隱私權或資料權益；
* 繞過平台風控、限流、計費或安全機制；
* 批量濫發、垃圾行銷、虛假客服等濫用行為；
* 違反第三方模型服務商政策之行為。

平台有權對異常流量、高風險請求或違規行為進行限制、封禁或終止服務。

## 4. 資料、隱私與日誌政策

本平台重視使用者隱私與資料安全。

除維持服務運行、安全風控、計費統計與法律合規所必要之技術資訊外，平台原則上不主動長期保存使用者 Prompt、上傳內容、模型輸出或業務資料。

本平台不將使用者資料用於：
* AI 模型訓練；
* 模型微調；
* 廣告行銷；
* 商業出售或資料交易。

但於以下情況，平台可能進行必要的安全稽核與日誌分析：
* 系統安全事件；
* 攻擊與濫用行為；
* 異常資源消耗；
* 法律、監管或司法要求。

部分服務可能由第三方模型供應商處理，相關資料流轉與保留規則，亦可能受第三方服務政策約束。

## 5. 第三方模型與供應商說明

本平台部分能力來自第三方 AI 模型與雲服務供應商，包括但不限於：
* OpenAI
* Anthropic
* Google
* xAI
* DeepSeek

使用者理解並同意：
* 第三方模型之輸出內容、穩定性、上下文保留、政策限制與可用性，不受本平台完全控制；
* 上游供應商可能隨時調整價格、限額、能力或政策；
* 因第三方服務異常、中斷、封鎖、政策調整或不可抗力造成之損失，本平台不承擔直接責任。

## 6. AI 生成內容說明

AI 模型輸出內容具有機率性、不確定性與幻覺風險。

平台不保證生成內容之：
* 真實性；
* 準確性；
* 合法性；
* 完整性；
* 適用性；
* 不侵權性。

使用者應自行判斷並承擔因使用 AI 生成內容所產生之一切風險與責任。

若法律或監管要求，平台有權對 AI 生成內容進行必要標識、風控或限制。

## 7. API Key 與帳戶安全

使用者應妥善保管帳戶、API Key、Access Token 與相關憑證。

因使用者自身原因導致之：
* Key 外洩；
* 帳戶被盜；
* 非授權調用；
* 資源濫用；
* 費用損失；

均由使用者自行承擔責任。

平台有權對異常請求、高風險 IP、可疑流量或攻擊行為進行限制、凍結或封禁。

## 8. 計費、儲值與退款

平台依公示之價格與計費規則提供服務。

Token 消耗、API 呼叫次數、頻寬與資源統計，以平台系統記錄為準。

除法律另有規定外：
* 已消耗之資源原則上不予退款；
* 因第三方模型異常、網路波動或不可抗力導致之部分失敗請求，平台可依實際情況進行調整或補償；
* 平台有權依營運需要調整價格、套餐或資源策略。

## 9. 服務可用性

平台將盡力維持服務穩定與高可用性，但不保證服務絕對不中斷。

平台有權基於：
* 系統維護；
* 容量調整；
* 安全風控；
* 上游供應變更；
* 法律合規要求；

對部分模型、介面或功能進行升級、限流、暫停或下線。

## 10. 協議變更

平台有權依業務發展、法律法規或合規要求，對本協議進行更新。

更新後內容於平台公告後生效，使用者繼續使用服務即視為接受更新後協議。

## 11. 協議終止

若使用者存在以下情況，平台有權暫停或終止服務：
* 違反本協議；
* 存在違法或高風險行為；
* 濫用平台資源；
* 影響平台安全或穩定運行；
* 違反第三方供應商政策。

平台並保留追究相關責任之權利。

## 12. 其他說明

本協議之訂定、執行與解釋，依適用法律原則及國際商業慣例進行。

若本協議部分條款被認定無效，不影響其他條款之效力。
`
