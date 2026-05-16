package service

// defaultTermsContentEN 与 defaultTermsContentZhTW 是平台首次启用登录条款时使用的默认文案。
// 仅在管理员尚未保存自定义条款时返回；已配置的条款不会被覆盖。
//
// 改动须知：修改文案后请同步更新 login_agreement_updated_at 以触发用户重新同意。

const defaultTermsContentEN = `## 1. Acceptance of Terms

By accessing or using this platform (the "Service"), you agree to be bound by these Terms of Service ("Terms"). If you do not agree to these Terms, do not use the Service.

These Terms constitute a binding agreement between you ("User") and the platform operator ("we", "us", "platform").

## 2. Service Description

The platform provides aggregated access to multiple third-party AI model providers and related developer APIs through a unified gateway, API keys, account management, and usage analytics.

The Service is provided on an as-is, as-available basis. Specific models, providers, features, and quotas may change without prior notice based on upstream availability and platform configuration.

## 3. Eligibility & Account Registration

You must be legally capable of entering into a binding contract under your jurisdiction.

Accurate and up-to-date registration information is required. You are responsible for keeping your account information current.

The Service is not directed at users located in mainland China. By registering, you confirm that you are not accessing the Service from a jurisdiction where its provision is prohibited.

## 4. Acceptable Use

You agree NOT to use the Service to:
* Violate any applicable laws, regulations, or third-party rights;
* Generate, distribute, or facilitate illegal, harmful, defamatory, or deceptive content;
* Generate content that infringes intellectual property, privacy, or publicity rights;
* Attempt to reverse-engineer, scrape, or interfere with platform infrastructure;
* Resell or redistribute Service capacity in violation of upstream provider policies;
* Circumvent rate limits, quotas, billing, or security controls;
* Submit personal data of others without lawful basis.

The platform may, at its discretion, monitor traffic patterns for abuse prevention.

## 5. Intellectual Property

Platform branding, source code, configurations, documentation, and aggregated analytics are owned by the platform operator and its licensors.

You retain ownership of inputs you submit. AI-generated outputs are subject to the licenses and terms of the underlying model provider; the platform makes no additional ownership claims over them.

## 6. Content Disclaimer

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

const defaultTermsContentZhTW = `## 1. 條款的接受

存取或使用本平台（以下簡稱「服務」）即表示您同意受本服務條款（以下簡稱「條款」）的約束。如您不同意本條款，請勿使用本服務。

本條款構成您（「使用者」）與平台營運方（「我們」、「平台」）之間具有約束力的協議。

## 2. 服務說明

本平台透過統一閘道、API 金鑰、帳號管理與用量分析，為多家第三方 AI 模型供應商與相關開發者 API 提供整合存取。

本服務以「現狀」與「現有可用」基礎提供。具體的模型、供應商、功能與配額可能基於上游可用性與平台設定而變動，恕不另行通知。

## 3. 使用資格與帳號註冊

您必須具備在所屬司法管轄區內訂立具約束力契約的法律行為能力。

您須提供準確且最新的註冊資訊，並有責任維持帳號資訊的即時性。

本服務未針對位於中國大陸的使用者提供。註冊即表示您確認自己並非從禁止提供本服務的司法管轄區存取本服務。

## 4. 可接受的使用方式

您同意不將本服務用於：
* 違反任何適用法律、法規或第三方權利；
* 產生、散佈或協助散佈違法、有害、誹謗或欺騙性內容；
* 產生侵害智慧財產權、隱私或公開權的內容；
* 嘗試逆向工程、爬取或干擾平台基礎設施；
* 違反上游供應商政策轉售或再分發服務容量；
* 規避速率限制、配額、計費或安全控制；
* 在無合法基礎的情況下提交他人個人資料。

平台得基於濫用防護目的，自行決定監測流量模式。

## 5. 智慧財產權

平台品牌、原始碼、組態、文件與彙總分析資料，歸平台營運方與其授權方所有。

您保留對所提交輸入內容的所有權。AI 產生之輸出內容受底層模型供應商之授權與條款規範；平台不對其主張任何額外所有權。

## 6. 內容免責聲明

平台對於下列事項不作任何陳述或保證：
* 準確性；
* 真實性；
* 合法性；
* 完整性；
* 可靠性；
* 對特定用途的適用性；
* 不侵權性。

使用者須自行負責評估與使用 AI 產生之內容，並承擔一切相關風險與責任。

於法律或法規有所要求時，平台得套用內容標示、安全控制或輸出限制。

## 7. 帳號與 API 金鑰安全

使用者有責任妥善保管其帳號、API 金鑰、存取權杖與相關憑證。

下列情形所生之損失或責任：
* 憑證洩漏；
* 未經授權的存取；
* API 金鑰遭濫用；
* 帳號遭入侵；
* 未經授權的使用費用；

概由使用者自行承擔。

平台保留對可疑流量、高風險請求、異常 API 活動或潛在攻擊，予以暫停或限制的權利。

## 8. 計費、額度與退費

服務依平台公佈之定價與用量政策計費。

Token 用量、API 呼叫次數、頻寬用量及相關統計，均以平台系統紀錄為準。

除適用法律另有規定外：
* 已消耗之資源一般不予退費；
* 對於上游中斷或技術異常導致的請求失敗，平台得自行決定提供調整或補償；
* 定價、配額、方案與計費結構得隨時更新。

## 9. 服務可用性

平台將盡商業上合理之努力以維持服務之穩定與高可用，但不保證服務不中斷。

平台保留基於下列原因升級、限制、暫停或停止任何模型、API 或功能之權利：
* 系統維護；
* 容量調整；
* 安全與風險控制；
* 上游供應商變更；
* 法律或法規要求。

## 10. 條款的修訂

平台保留基於業務、法律、法規或合規要求，隨時更新或修改本條款之權利。

更新後的條款於平台公佈時生效。繼續使用服務即視為接受修訂後的條款。

## 11. 暫停與終止

如使用者有下列情形，平台保留暫停或終止服務之權利：
* 違反本條款；
* 從事違法或高風險活動；
* 濫用平台資源；
* 危及平台安全或運作穩定；
* 違反上游供應商政策。

平台並保留依適用情形採取一切法律救濟之權利。

## 12. 一般條款

本條款應依適用之法律原則與國際商業慣例解釋。

若本條款任一條文被認定為無效或不可執行，其餘條文仍應繼續完全有效。
`
