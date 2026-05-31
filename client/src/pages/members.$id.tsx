import Layout from "../../components/layout";
import { useParams } from '@tanstack/react-router';
import { ChangeEvent, useEffect, useMemo, useRef, useState } from "react";
import StatusBadges from "../../components/statusbadges";
import MemberRepo from "../../repository/MemberRepo";
import HPProfileRepo, { validatePhotoFile } from "../../repository/HPProfileRepo";
import Member from "../../models/Member";
import HPProfile, { emptyHPProfile, HIDDEN_FIELD_KEYS, HiddenFieldKey } from "../../models/HPProfile";
import { useAppContext } from "../context";

export default function MemberView() {
  const { myself } = useAppContext();
  const repo = useMemo(() => new MemberRepo(), []);
  const { id } = useParams({ strict: false });
  const [member, setMember] = useState<Member>(null);
  const [num, setNumberInput] = useState<number>(null);
  useEffect(() => { if (id) repo.get(id).then(setMember); }, [id, repo]);
  if (!member) return <></>;

  const isOwnPage = myself?.slack?.id === member.slack.id;

  return (
    <Layout>
      <div className="flex flex-col sm:flex-row sm:space-x-4">
        <div className="w-full sm:w-44 mb-4 sm:mb-0">
          <img
            className="rounded-md w-full sm:w-44"
            src={member.slack.profile.image_512} alt={member.slack.profile.name}
          />
        </div>
        <div className="flex-grow">
          <div className="flex flex-col h-full">
            <h1 className="text-3xl font-medium">{member.slack.profile.real_name}</h1>
            <div className="text-2xl flex-grow text-gray-800">{member.slack.profile.title || "ポジション未設定"}</div>
            <div className="text-xs text-gray-400 mt-1">名前・アイコン・ポジションは Slack プロフィールから同期されます</div>
          </div>
        </div>
      </div>

      <hr className="my-4" />

      <div className="py-2">
        <div className="form-group flex items-center space-x-4">
          <span>背番号:</span>
          <input type="number"
            defaultValue={member.number || num}
            onChange={ev => setNumberInput(parseInt(ev.target.value))}
            className="flex-grow form-input border-transparent bg-gray-100 rounded-md"
            placeholder="0~99を選択"
            min="0" max="99" step="1"
          />
          <button
            role="button" className="border rounded-md px-4 py-2 cursor-pointer"
            onClick={() => repo.update(member.slack.id, { number: num })}
          >設定</button>
        </div>
      </div>
      <div className="py-2">
        <div className="flex space-x-2"><StatusBadges member={member} size="text-lg px-4 py-1" /></div>
      </div>

      {myself.slack.is_admin ? <AdminMenu member={member} repo={repo} /> : null}

      <hr className="my-4" />

      {isOwnPage && <HPProfileSection memberId={member.slack.id} />}

      <div className="p-8 flex justify-center items-center">
        <a href="/members" className="underline">一覧に戻る</a>
      </div>
    </Layout>
  )
}

function AdminMenu({ member, repo }: { member: Member, repo: MemberRepo }) {
  const { status, slack } = member;
  const onInputChange = async (ev: ChangeEvent<HTMLSelectElement>) => {
    await repo.update(slack.id, { status: ev.target.value });
  };
  return <div className="p-2 border rounded-md bg-red-100">
    <h3>管理者メニュー</h3>
    <div>
      <select
        className="w-full rounded-sm" defaultValue={slack.deleted ? "deleted" : (status || "active")}
        disabled={slack.deleted}
        onChange={onInputChange}
      >
        <option value="active">通常部員</option>
        <option value="limited">練習外部員（出欠回答不要）</option>
        <option value="inactive">休眠</option>
        <option
          value="deleted"
          disabled={!slack.deleted}
        >退部済み（Slackで設定）</option>
      </select>
    </div>
  </div>;
}

function Switch({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onChange}
      className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none ${
        checked ? "bg-blue-500" : "bg-gray-300"
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition duration-200 ease-in-out ${
          checked ? "translate-x-5" : "translate-x-0"
        }`}
      />
    </button>
  );
}

const FIELD_LABELS: Record<string, string> = {
  display_name: "表示名（スタッフはイニシャルや偽名も可）",
  display_name_kana: "表示名かな",
  first_name: "FirstName",
  family_name: "FamilyName",
  height: "身長 (cm)",
  weight: "体重 (kg)",
  position: "ポジション",
  hometown: "出身地",
  school: "出身校",
  faculty: "学部・学科",
  bio: "ひとこと",
};

function HPProfileSection({ memberId }: { memberId: string }) {
  const repo = useMemo(() => new HPProfileRepo(), []);
  const [profile, setProfile] = useState<HPProfile>(emptyHPProfile());
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const formalRef = useRef<HTMLInputElement>(null);
  const casualRef = useRef<HTMLInputElement>(null);
  const additionalRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    repo.get(memberId).then(setProfile).catch(() => {});
  }, [memberId, repo]);

  const toggleHiddenField = (key: HiddenFieldKey) => {
    setProfile(p => {
      const fields = p.hidden_fields || [];
      return {
        ...p,
        hidden_fields: fields.includes(key)
          ? fields.filter(f => f !== key)
          : [...fields, key],
      };
    });
  };

  const isHidden = (key: HiddenFieldKey) => (profile.hidden_fields || []).includes(key);

  const handleSave = async () => {
    setSaving(true);
    setMessage("");
    try {
      const saved = await repo.update(memberId, profile);
      setProfile(saved);
      setMessage("保存しました");
    } catch (e) {
      setMessage("保存に失敗しました: " + (e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handlePhotoUpload = async (
    type: "formal" | "casual" | "additional",
    file: File
  ) => {
    setMessage("アップロード中...");
    try {
      const { url } = await repo.uploadPhoto(memberId, type, file);
      setProfile(p => ({
        ...p,
        portrait_formal_url: type === "formal" ? url : p.portrait_formal_url,
        portrait_casual_url: type === "casual" ? url : p.portrait_casual_url,
        additional_photo_urls: type === "additional"
          ? [...(p.additional_photo_urls || []), url]
          : p.additional_photo_urls,
      }));
      setMessage("写真をアップロードしました");
    } catch (e) {
      setMessage("アップロードに失敗しました: " + (e as Error).message);
    }
  };

  return (
    <div className="mt-2 border rounded-md p-4">
      <h2 className="text-xl font-semibold mb-3">HPプロフィール編集</h2>

      {/* 全体掲載トグル */}
      <button
        type="button"
        onClick={() => setProfile(p => ({ ...p, hide_from_hp: !p.hide_from_hp }))}
        className={`w-full flex items-center justify-between p-3 rounded-xl border-2 transition-colors mb-5 text-left ${
          profile.hide_from_hp
            ? "border-gray-200 bg-gray-50"
            : "border-blue-200 bg-blue-50"
        }`}
      >
        <div>
          <div className={`text-sm font-semibold ${profile.hide_from_hp ? "text-gray-500" : "text-blue-700"}`}>
            {profile.hide_from_hp ? "非掲載" : "HP掲載中"}
          </div>
          <div className="text-xs text-gray-500 mt-0.5">
            {profile.hide_from_hp
              ? "このプロフィールはHPに掲載されていません"
              : "このプロフィールはHPに掲載されています"}
          </div>
        </div>
        <Switch
          checked={!profile.hide_from_hp}
          onChange={() => setProfile(p => ({ ...p, hide_from_hp: !p.hide_from_hp }))}
        />
      </button>

      <div className={profile.hide_from_hp ? "opacity-40 pointer-events-none" : ""}>
        {/* テキストフィールド: ラベル+スイッチ行 → 入力行 */}
        <div className="grid grid-cols-1 gap-4 mb-5">
          {(Object.keys(FIELD_LABELS) as HiddenFieldKey[]).map(key => (
            <div key={key}>
              <div className="flex items-center justify-between mb-1">
                <label className="text-sm text-gray-700 pr-2 leading-tight">{FIELD_LABELS[key]}</label>
                <div className="flex items-center gap-1.5 shrink-0">
                  <span className="text-xs text-gray-500">{isHidden(key) ? "非掲載" : "掲載"}</span>
                  <Switch checked={!isHidden(key)} onChange={() => toggleHiddenField(key)} />
                </div>
              </div>
              {key === "bio" ? (
                <textarea
                  className="w-full form-input border border-gray-200 bg-gray-50 rounded-md text-sm p-2"
                  rows={2}
                  value={(profile[key as keyof HPProfile] as string) || ""}
                  onChange={e => setProfile(p => ({ ...p, [key]: e.target.value }))}
                />
              ) : (
                <input
                  type={key === "height" || key === "weight" ? "number" : "text"}
                  className="w-full form-input border border-gray-200 bg-gray-50 rounded-md text-sm p-2"
                  value={(profile[key as keyof HPProfile] as string | number) || ""}
                  onChange={e => setProfile(p => ({
                    ...p,
                    [key]: key === "height" || key === "weight"
                      ? parseInt(e.target.value) || 0
                      : e.target.value,
                  }))}
                />
              )}
            </div>
          ))}
        </div>

        {/* 写真 */}
        <div className="mb-5 space-y-3">
          <h3 className="text-sm font-medium text-gray-700">プロフィール写真</h3>

          {/* ポートレイト */}
          <div className="border border-gray-200 rounded-xl p-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-gray-700">ポートレイト <span className="text-red-500">*</span></span>
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-gray-500">{isHidden("portrait_formal") ? "非掲載" : "掲載"}</span>
                <Switch checked={!isHidden("portrait_formal")} onChange={() => toggleHiddenField("portrait_formal")} />
              </div>
            </div>
            <div className="flex items-center gap-3">
              {profile.portrait_formal_url && (
                <img src={profile.portrait_formal_url} className="w-14 h-14 object-cover rounded-lg shrink-0" alt="ポートレイト" />
              )}
              <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={formalRef} className="hidden"
                onChange={e => e.target.files?.[0] && handlePhotoUpload("formal", e.target.files[0])} />
              <button type="button" className="flex-grow text-sm border border-gray-300 rounded-lg px-3 py-2 text-gray-600 bg-white"
                onClick={() => formalRef.current?.click()}>
                {profile.portrait_formal_url ? "写真を変更" : "写真をアップロード"}
              </button>
            </div>
          </div>

          {/* カジュアルポートレイト */}
          <div className="border border-gray-200 rounded-xl p-3">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm text-gray-700">カジュアルポートレイト <span className="text-red-500">*</span></span>
              <div className="flex items-center gap-1.5">
                <span className="text-xs text-gray-500">{isHidden("portrait_casual") ? "非掲載" : "掲載"}</span>
                <Switch checked={!isHidden("portrait_casual")} onChange={() => toggleHiddenField("portrait_casual")} />
              </div>
            </div>
            <div className="flex items-center gap-3">
              {profile.portrait_casual_url && (
                <img src={profile.portrait_casual_url} className="w-14 h-14 object-cover rounded-lg shrink-0" alt="カジュアルポートレイト" />
              )}
              <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={casualRef} className="hidden"
                onChange={e => e.target.files?.[0] && handlePhotoUpload("casual", e.target.files[0])} />
              <button type="button" className="flex-grow text-sm border border-gray-300 rounded-lg px-3 py-2 text-gray-600 bg-white"
                onClick={() => casualRef.current?.click()}>
                {profile.portrait_casual_url ? "写真を変更" : "写真をアップロード"}
              </button>
            </div>
          </div>

          {/* 追加スナップ写真（掲載/非掲載スイッチなし） */}
          <div className="border border-gray-200 rounded-xl p-3">
            <div className="text-sm text-gray-700 mb-2">追加スナップ写真</div>
            <div className="flex flex-wrap gap-2 mb-2">
              {(profile.additional_photo_urls || []).map((url, i) => (
                <img key={i} src={url} className="w-14 h-14 object-cover rounded-lg" alt={`スナップ${i + 1}`} />
              ))}
            </div>
            <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={additionalRef} className="hidden"
              onChange={e => e.target.files?.[0] && handlePhotoUpload("additional", e.target.files[0])} />
            <button type="button" className="text-sm border border-gray-300 rounded-lg px-3 py-2 text-gray-600 bg-white"
              onClick={() => additionalRef.current?.click()}>
              写真を追加
            </button>
          </div>
        </div>

        <button
          className="bg-blue-600 text-white rounded-md px-6 py-2 disabled:opacity-50"
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? "保存中..." : "保存"}
        </button>
        {message && <p className="mt-2 text-sm text-gray-700">{message}</p>}
      </div>
    </div>
  );
}
