import { FormEvent, useMemo, useState } from "react";
import ApplicationRepo from "../../repository/ApplicationRepo";

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

const CONSENT_TEXT = `━━━━━━━━━━━━━━
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

const INITIAL_FIELDS: Record<string, string> = {
  family_name: "",
  given_name: "",
  family_name_kana: "",
  given_name_kana: "",
  family_name_alphabet: "",
  given_name_alphabet: "",
  date_of_birth: "",
  phone: "",
  emergency_phone: "",
  emergency_name: "",
  emergency_relation: "",
  university: "",
  hometown: "",
  workplace: "",
  previous_team: "",
  gmail: "",
  extra_email: "",
  role: "",
  position: "",
  height: "",
  weight: "",
};

const personalFields = [
  { key: "family_name", label: "姓", type: "text" },
  { key: "given_name", label: "名", type: "text" },
  { key: "family_name_kana", label: "姓（カナ）", type: "text" },
  { key: "given_name_kana", label: "名（カナ）", type: "text" },
  { key: "family_name_alphabet", label: "姓（アルファベット）", type: "text" },
  { key: "given_name_alphabet", label: "名（アルファベット）", type: "text" },
  { key: "date_of_birth", label: "生年月日", type: "date" },
  { key: "phone", label: "電話番号", type: "tel" },
  { key: "emergency_phone", label: "緊急連絡先電話番号", type: "tel" },
  { key: "emergency_name", label: "緊急連絡先氏名", type: "text" },
  { key: "emergency_relation", label: "緊急連絡先続柄", type: "text" },
  { key: "university", label: "出身大学", type: "text" },
  { key: "workplace", label: "勤務先", type: "text" },
  { key: "previous_team", label: "前所属チーム", type: "text" },
  { key: "gmail", label: "Gmail", type: "email" },
  { key: "extra_email", label: "追加メールアドレス", type: "email" },
];

export default function OnboardingForm() {
  const repo = useMemo(() => new ApplicationRepo(), []);
  const [fields, setFields] = useState<Record<string, string>>(INITIAL_FIELDS);
  const [agreed, setAgreed] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState("");

  const setField = (key: string, value: string) => {
    setFields(prev => ({ ...prev, [key]: value }));
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError("");
    if (!agreed) {
      setError("同意事項への同意が必要です。");
      return;
    }
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
      setError(err instanceof Error ? err.message : "送信に失敗しました。");
    } finally {
      setSubmitting(false);
    }
  };

  if (submitted) {
    return (
      <main className="min-h-screen bg-gray-50 px-4 py-12">
        <div className="mx-auto max-w-2xl rounded-lg border border-gray-200 bg-white p-8 shadow-sm">
          <p className="text-sm font-semibold text-blue-700">入部申請</p>
          <h1 className="mt-2 text-2xl font-bold text-gray-900">送信が完了しました</h1>
          <p className="mt-4 text-sm leading-6 text-gray-600">
            内容を確認後、担当者から連絡します。
          </p>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-gray-50 px-4 py-8">
      <form onSubmit={submit} className="mx-auto max-w-4xl space-y-6">
        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <p className="text-sm font-semibold text-blue-700">TRIAX 入部申請</p>
          <h1 className="mt-2 text-2xl font-bold text-gray-900">入部申請フォーム</h1>
        </section>

        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">同意事項</h2>
          <pre className="mt-4 max-h-96 overflow-y-auto whitespace-pre-wrap rounded-md bg-gray-50 p-4 text-sm leading-6 text-gray-700">{CONSENT_TEXT}</pre>
          <label className="mt-4 flex items-start gap-3 text-sm text-gray-800">
            <input
              type="checkbox"
              className="mt-1 h-5 w-5 rounded border-gray-300"
              checked={agreed}
              onChange={event => setAgreed(event.target.checked)}
              required
            />
            <span>第1条から第8条までの内容を確認し、同意します。</span>
          </label>
        </section>

        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">基本情報</h2>
          <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            {personalFields.map(field => (
              <TextField
                key={field.key}
                label={field.label}
                type={field.type}
                value={fields[field.key]}
                onChange={value => setField(field.key, value)}
                required={field.key !== "extra_email" && field.key !== "previous_team"}
              />
            ))}
            <SelectField
              label="出身地"
              value={fields.hometown}
              options={PREFECTURES}
              onChange={value => setField("hometown", value)}
              required
            />
            <SelectField
              label="区分"
              value={fields.role}
              options={["選手", "スタッフ・運営"]}
              onChange={value => setField("role", value)}
              required
            />
          </div>
        </section>

        {fields.role === "選手" && (
          <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
            <h2 className="text-lg font-semibold text-gray-900">選手情報</h2>
            <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-3">
              <SelectField
                label="ポジション"
                value={fields.position}
                options={POSITIONS}
                onChange={value => setField("position", value)}
                required
              />
              <TextField
                label="身長（cm）"
                type="number"
                value={fields.height}
                onChange={value => setField("height", value)}
                required
              />
              <TextField
                label="体重（kg）"
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
            {submitting ? "送信中..." : "申請を送信する"}
          </button>
        </div>
      </form>
    </main>
  );
}

function TextField({
  label,
  type,
  value,
  onChange,
  required,
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
        onChange={event => onChange(event.target.value)}
        required={required}
      />
    </label>
  );
}

function SelectField({
  label,
  value,
  options,
  onChange,
  required,
}: {
  label: string;
  value: string;
  options: string[];
  onChange: (value: string) => void;
  required?: boolean;
}) {
  return (
    <label className="block text-sm font-medium text-gray-700">
      {label}
      <select
        className="mt-1 block w-full rounded-md border-gray-300 text-sm shadow-sm focus:border-blue-600 focus:ring-blue-600"
        value={value}
        onChange={event => onChange(event.target.value)}
        required={required}
      >
        <option value="">選択してください</option>
        {options.map(option => (
          <option key={option} value={option}>{option}</option>
        ))}
      </select>
    </label>
  );
}
