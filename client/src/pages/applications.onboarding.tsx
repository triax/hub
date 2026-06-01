import { FormEvent, useMemo, useState } from "react";
import ApplicationRepo from "../../repository/ApplicationRepo";

type Lang = "ja" | "en";

const PREFECTURES = [
  "北海道", "青森県", "岩手県", "宮城県", "秋田県", "山形県", "福島県",
  "茨城県", "栃木県", "群馬県", "埼玉県", "千葉県", "東京都", "神奈川県",
  "新潟県", "富山県", "石川県", "福井県", "山梨県", "長野県", "岐阜県",
  "静岡県", "愛知県", "三重県", "滋賀県", "京都府", "大阪府", "兵庫県",
  "奈良県", "和歌山県", "鳥取県", "島根県", "岡山県", "広島県", "山口県",
  "徳島県", "香川県", "愛媛県", "高知県", "福岡県", "佐賀県", "長崎県",
  "熊本県", "大分県", "宮崎県", "鹿児島県", "沖縄県", "その他",
];

const POSITIONS = ["QB", "RB", "WR", "TE", "OL", "DL", "LB", "DB", "K/P", "LS/SS", "その他"];

const CONSENT_JA = `━━━━━━━━━━━━━━
第1条 入部資格
チームはNFA（一般社団法人日本社会人アメリカンフットボール協会）に加盟しており、選手・スタッフの登録はNFA登録規程（2024年改訂版）に基づきます。入部にあたり、以下の要件をすべて満たすことを確認します。
①入部時点で満18歳以上であり、日本国内に居住する社会人（大学院生を含む）であること
②現在、他のNFA加盟チームに選手またはスタッフとして登録していないこと（二重登録は禁止されています）
③Xリーグ公式戦への出場には、別途満20歳以上であることが必要です
なお、外国籍の方は在日米軍軍人・軍属でなく、NFLプロフットボール経験者でないことが要件となります。

━━━━━━━━━━━━━━
第2条 活動に伴うリスクと責任
アメリカンフットボールは身体的接触（コンタクト）を伴う競技です。
【保険加入】チームはNFA登録規程第7条に基づき、登録選手を対象として死亡・傷害を補償する保険に加入します。
【医療サポート】活動中の怪我に対し、チームドクターおよびトレーナーによるサポートを受けることができます。ただし、このサポートは応急的・補助的なものです。
【免責事項】活動中に生じた怪我・事故・疾病については、チームの故意または重大な過失によるものを除き、保険補償の範囲を超えた損害賠償等の責任をチームは負いません。

━━━━━━━━━━━━━━
第3条 育成選手の定義
「育成選手」とは、チームに所属しながらも、Xリーグの公式選手登録を行わない選手を指します。育成選手はXリーグ公式戦に出場できません。部費の対象外となります。

━━━━━━━━━━━━━━
第4条 チーム活動費用について
部費は秋シーズンにXリーグの公式選手として登録される方（登録選手）を対象とします。スタッフおよび育成選手は対象外です。部費の金額はシーズン終了時に決定・通知します。登録初年度は割引があります（2025年実績：6割引）。

━━━━━━━━━━━━━━
第5条 移籍の制限
入部後は、チームが移籍を承認した場合を除き、NFA加盟の他チームへ移籍することはできません。チームは移籍希望者からの申し出を受けた場合、総合的に考慮した上で承認の可否を判断します。

━━━━━━━━━━━━━━
第6条 NFA登録規程への同意
選手・スタッフの登録はNFA登録規程（2024年改訂版）に基づきます。登録は年1回、登録抹消はいつでも申請できますが同一年度内の再登録はできません。

━━━━━━━━━━━━━━
第7条 個人情報の取り扱い
入部時に提供いただいた個人情報は、チーム運営およびNFA登録手続きの目的にのみ使用します。NFA・Xリーグへの登録手続きに必要な範囲での第三者提供について、あらかじめ同意いただきます。

━━━━━━━━━━━━━━
第8条 チームルール・行動規範の遵守
チームの定める規則・行動規範を遵守し、チームおよびリーグの名誉を損なう行為を行わないことに同意します。`;

const CONSENT_EN = `━━━━━━━━━━━━━━
Article 1  Eligibility for Membership
The team is a member of the NFA (Japan American Football Association), and registration of players and staff is conducted in accordance with the NFA Registration Regulations (2024 revised edition). Upon joining, please confirm all of the following requirements are satisfied.
① You are at least 18 years of age and are a member of society residing in Japan (including graduate students).
② You are not currently registered with any other NFA-affiliated team as a player or staff member (dual registration is prohibited).
③ Participation in official X League games requires you to be at least 20 years of age.
Non-Japanese nationals must not be a member of the U.S. military or a military civilian employee stationed in Japan, and must not have professional playing experience in the NFL.

━━━━━━━━━━━━━━
Article 2  Risks and Responsibilities
American football is a sport involving physical contact. By joining, you agree to the following.
[Insurance] The team will enroll registered players in insurance covering death and injury in accordance with Article 7 of the NFA Registration Regulations.
[Medical Support] Support from the team doctor and trainer is available during activities. This support is supplementary and does not replace medical treatment.
[Disclaimer] Except in cases of intentional misconduct or gross negligence on the part of the team, the team shall not be liable for damages arising from injuries, accidents, or illness beyond the scope of insurance coverage.

━━━━━━━━━━━━━━
Article 3  Definition of Development Players
A "Development Player" is a player who belongs to the team but is not officially registered in the X League. Development Players may not participate in official X League games and are exempt from the membership fee.

━━━━━━━━━━━━━━
Article 4  Team Activity Fee
The membership fee applies to players officially registered in the X League for the fall season. Staff and Development Players are exempt. The amount is determined at the end of the season. A discount applies in the first year of registration (2025 actual result: 60% discount).

━━━━━━━━━━━━━━
Article 5  Transfer Restrictions
After joining, no member may transfer to another NFA-affiliated team without the team's approval. Transfer requests will be reviewed comprehensively, taking into account tactical impact, length of service, and activity record.

━━━━━━━━━━━━━━
Article 6  Consent to NFA Registration Regulations
Player and staff registration is conducted in accordance with the NFA Registration Regulations (2024 revised edition). Registration is conducted once per year. Cancellation may be requested at any time, but re-registration within the same year is not permitted.

━━━━━━━━━━━━━━
Article 7  Handling of Personal Information
Personal information provided at the time of joining will be used solely for team management and NFA registration purposes. You consent in advance to the provision of such information to third parties (NFA, X League) to the extent necessary for registration.

━━━━━━━━━━━━━━
Article 8  Compliance with Team Rules and Code of Conduct
You agree to comply with the team's rules and code of conduct and not to engage in any conduct that would damage the reputation of the team or the league.`;

const T = {
  ja: {
    badge: "TRIAX 入部申請",
    title: "入部申請フォーム",
    consentTitle: "同意事項",
    consentText: CONSENT_JA,
    consentCheck: "第1条から第8条までの内容を確認し、同意します。",
    basicInfoTitle: "基本情報",
    playerInfoTitle: "選手情報",
    placeholder: "選択してください",
    submit: "申請を送信する",
    submitting: "送信中...",
    consentRequired: "同意事項への同意が必要です。",
    submitError: "送信に失敗しました。",
    successBadge: "入部申請",
    successTitle: "送信が完了しました",
    successBody: "内容を確認後、担当者から連絡します。",
    fields: {
      family_name: "姓",
      given_name: "名",
      family_name_kana: "姓（カナ）",
      given_name_kana: "名（カナ）",
      family_name_alphabet: "姓（アルファベット）",
      given_name_alphabet: "名（アルファベット）",
      date_of_birth: "生年月日",
      phone: "電話番号",
      emergency_phone: "緊急連絡先電話番号",
      emergency_name: "緊急連絡先氏名",
      emergency_relation: "緊急連絡先続柄",
      university: "出身大学",
      workplace: "勤務先",
      previous_team: "前所属チーム（1年以内）",
      gmail: "Gmailアドレス",
      extra_email: "追加メールアドレス（任意）",
      hometown: "出身地（都道府県）",
      role: "区分",
      position: "希望ポジション",
      height: "身長（cm）",
      weight: "体重（kg）",
    } as Record<string, string>,
    roles: [
      { value: "選手", label: "選手" },
      { value: "スタッフ・運営", label: "スタッフ・運営" },
    ],
  },
  en: {
    badge: "TRIAX Membership Application",
    title: "Application Form",
    consentTitle: "Terms & Conditions",
    consentText: CONSENT_EN,
    consentCheck: "I have read and agree to all provisions in Articles 1 through 8.",
    basicInfoTitle: "Personal Information",
    playerInfoTitle: "Player Information",
    placeholder: "Select...",
    submit: "Submit Application",
    submitting: "Submitting...",
    consentRequired: "Please agree to the terms and conditions.",
    submitError: "Submission failed.",
    successBadge: "Membership Application",
    successTitle: "Submission Complete",
    successBody: "We will review your application and contact you shortly.",
    fields: {
      family_name: "Last Name",
      given_name: "First Name",
      family_name_kana: "Last Name (Kana)",
      given_name_kana: "First Name (Kana)",
      family_name_alphabet: "Last Name (Alphabet)",
      given_name_alphabet: "First Name (Alphabet)",
      date_of_birth: "Date of Birth",
      phone: "Phone Number",
      emergency_phone: "Emergency Contact Phone",
      emergency_name: "Emergency Contact Name",
      emergency_relation: "Relationship",
      university: "University",
      workplace: "Employer",
      previous_team: "Previous Team (within 1 year, optional)",
      gmail: "Gmail Address",
      extra_email: "Additional Email (optional)",
      hometown: "Prefecture of Origin",
      role: "Role",
      position: "Desired Position",
      height: "Height (cm)",
      weight: "Weight (kg)",
    } as Record<string, string>,
    roles: [
      { value: "選手", label: "Player" },
      { value: "スタッフ・運営", label: "Staff / Management" },
    ],
  },
};

const PERSONAL_FIELD_KEYS = [
  "family_name", "given_name",
  "family_name_kana", "given_name_kana",
  "family_name_alphabet", "given_name_alphabet",
  "date_of_birth",
  "phone", "emergency_phone", "emergency_name", "emergency_relation",
  "university", "workplace", "previous_team",
  "gmail", "extra_email",
];

const OPTIONAL_KEYS = new Set(["extra_email", "previous_team"]);

const FIELD_TYPE: Record<string, string> = {
  date_of_birth: "date",
  phone: "tel",
  emergency_phone: "tel",
  gmail: "email",
  extra_email: "email",
};

const INITIAL_FIELDS: Record<string, string> = {
  family_name: "", given_name: "",
  family_name_kana: "", given_name_kana: "",
  family_name_alphabet: "", given_name_alphabet: "",
  date_of_birth: "", phone: "", emergency_phone: "",
  emergency_name: "", emergency_relation: "",
  university: "", hometown: "", workplace: "",
  previous_team: "", gmail: "", extra_email: "",
  role: "", position: "", height: "", weight: "",
};

export default function OnboardingForm() {
  const repo = useMemo(() => new ApplicationRepo(), []);
  const [lang, setLang] = useState<Lang>("ja");
  const [fields, setFields] = useState<Record<string, string>>(INITIAL_FIELDS);
  const [agreed, setAgreed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");

  const t = T[lang];

  const setField = (key: string, value: string) =>
    setFields(prev => ({ ...prev, [key]: value }));

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    if (!agreed) { setError(t.consentRequired); return; }
    setSubmitting(true);
    try {
      await repo.submit({
        type: "onboarding",
        email: fields.gmail,
        name: `${fields.family_name}${fields.given_name}`,
        fields,
        consent_agreed_at: new Date().toISOString(),
      });
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : t.submitError);
    } finally {
      setSubmitting(false);
    }
  };

  if (submitted) {
    return (
      <main className="min-h-screen bg-gray-50 px-4 py-12">
        <div className="mx-auto max-w-2xl rounded-lg border border-gray-200 bg-white p-8 shadow-sm">
          <p className="text-sm font-semibold text-blue-700">{t.successBadge}</p>
          <h1 className="mt-2 text-2xl font-bold text-gray-900">{t.successTitle}</h1>
          <p className="mt-4 text-sm leading-6 text-gray-600">{t.successBody}</p>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-gray-50 px-4 py-8">
      <form onSubmit={submit} className="mx-auto max-w-4xl space-y-6">

        {/* ヘッダー + 言語切り替え */}
        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <p className="text-sm font-semibold text-blue-700">{t.badge}</p>
              <h1 className="mt-1 text-2xl font-bold text-gray-900">{t.title}</h1>
            </div>
            <button
              type="button"
              onClick={() => setLang(l => l === "ja" ? "en" : "ja")}
              className="shrink-0 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-50"
            >
              {lang === "ja" ? "English" : "日本語"}
            </button>
          </div>
        </section>

        {/* 同意事項 */}
        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">{t.consentTitle}</h2>
          <pre className="mt-4 max-h-96 overflow-y-auto whitespace-pre-wrap rounded-md bg-gray-50 p-4 text-sm leading-6 text-gray-700">
            {t.consentText}
          </pre>
          <label className="mt-4 flex items-start gap-3 text-sm text-gray-800">
            <input
              type="checkbox"
              className="mt-1 h-5 w-5 rounded border-gray-300"
              checked={agreed}
              onChange={e => setAgreed(e.target.checked)}
              required
            />
            <span>{t.consentCheck}</span>
          </label>
        </section>

        {/* 基本情報 */}
        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">{t.basicInfoTitle}</h2>
          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            {PERSONAL_FIELD_KEYS.map(key => (
              <TextField
                key={key}
                label={t.fields[key]}
                type={FIELD_TYPE[key] || "text"}
                value={fields[key]}
                onChange={value => setField(key, value)}
                required={!OPTIONAL_KEYS.has(key)}
              />
            ))}
            <SelectField
              label={t.fields.hometown}
              value={fields.hometown}
              options={PREFECTURES.map(p => ({ value: p, label: p }))}
              onChange={value => setField("hometown", value)}
              placeholder={t.placeholder}
              required
            />
            <SelectField
              label={t.fields.role}
              value={fields.role}
              options={t.roles}
              onChange={value => setField("role", value)}
              placeholder={t.placeholder}
              required
            />
          </div>
        </section>

        {/* 選手情報 */}
        {fields.role === "選手" && (
          <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
            <h2 className="text-lg font-semibold text-gray-900">{t.playerInfoTitle}</h2>
            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
              <SelectField
                label={t.fields.position}
                value={fields.position}
                options={POSITIONS.map(p => ({ value: p, label: p }))}
                onChange={value => setField("position", value)}
                placeholder={t.placeholder}
                required
              />
              <TextField
                label={t.fields.height}
                type="number"
                value={fields.height}
                onChange={value => setField("height", value)}
                required
              />
              <TextField
                label={t.fields.weight}
                type="number"
                value={fields.weight}
                onChange={value => setField("weight", value)}
                required
              />
            </div>
          </section>
        )}

        {error && (
          <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        <div className="sticky bottom-0 border-t border-gray-200 bg-gray-50 py-4">
          <button
            type="submit"
            className="w-full rounded-md bg-blue-700 px-4 py-3 text-sm font-semibold text-white disabled:opacity-50"
            disabled={submitting || !agreed}
          >
            {submitting ? t.submitting : t.submit}
          </button>
        </div>
      </form>
    </main>
  );
}

function TextField({
  label, type, value, onChange, required,
}: {
  label: string;
  type: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
}) {
  return (
    <label className="block text-sm font-medium text-gray-700">
      {label}
      <input
        type={type}
        className="mt-1 block w-full rounded-md border-gray-300 text-sm shadow-sm focus:border-blue-600 focus:ring-blue-600"
        value={value}
        onChange={e => onChange(e.target.value)}
        required={required}
      />
    </label>
  );
}

function SelectField({
  label, value, options, onChange, placeholder, required,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  placeholder: string;
  required?: boolean;
}) {
  return (
    <label className="block text-sm font-medium text-gray-700">
      {label}
      <select
        className="mt-1 block w-full rounded-md border-gray-300 text-sm shadow-sm focus:border-blue-600 focus:ring-blue-600"
        value={value}
        onChange={e => onChange(e.target.value)}
        required={required}
      >
        <option value="">{placeholder}</option>
        {options.map(o => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </label>
  );
}
