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
      <div className="flex space-x-4">
        <div className="fle w-44">
          <img
            className="rounded-md"
            src={member.slack.profile.image_512} alt={member.slack.profile.name}
          />
        </div>
        <div className="flex-grow">
          <div className="flex flex-col h-full">
            <h1 className="text-3xl font-medium">{member.slack.profile.real_name}</h1>
            <div className="text-2xl flex-grow text-gray-800">{member.slack.profile.title || "ポジション未設定"}</div>
            <div className="flex flex-row-reverse text-gray-400">Slackで編集可</div>
          </div>
        </div>
      </div>
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

      {isOwnPage && <HPProfileSection memberId={member.slack.id} />}

      <div className="p-12 flex justify-center items-center">
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

const FIELD_LABELS: Record<string, string> = {
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
    const err = validatePhotoFile(file);
    if (err) { setMessage(err); return; }
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
    <div className="mt-6 border rounded-md p-4">
      <h2 className="text-xl font-semibold mb-3">HPプロフィール編集</h2>

      <label className="flex items-center space-x-2 mb-4 text-red-600">
        <input
          type="checkbox"
          checked={profile.hide_from_hp}
          onChange={e => setProfile(p => ({ ...p, hide_from_hp: e.target.checked }))}
        />
        <span>HPに表示しない（すべての情報を非公開にする）</span>
      </label>

      <div className={profile.hide_from_hp ? "opacity-40 pointer-events-none" : ""}>
        {/* テキストフィールド */}
        <div className="grid grid-cols-1 gap-3 mb-4">
          {(Object.keys(FIELD_LABELS) as HiddenFieldKey[]).map(key => (
            <div key={key} className="flex items-center space-x-3">
              <label className="w-32 text-sm text-gray-600 shrink-0">{FIELD_LABELS[key]}</label>
              {key === "bio" ? (
                <textarea
                  className="flex-grow form-input border border-gray-200 bg-gray-50 rounded-md text-sm p-2"
                  rows={2}
                  value={(profile as Record<string, unknown>)[key] as string || ""}
                  onChange={e => setProfile(p => ({ ...p, [key]: e.target.value }))}
                />
              ) : (
                <input
                  type={key === "height" || key === "weight" ? "number" : "text"}
                  className="flex-grow form-input border border-gray-200 bg-gray-50 rounded-md text-sm p-2"
                  value={(profile as Record<string, unknown>)[key] as string | number || ""}
                  onChange={e => setProfile(p => ({
                    ...p,
                    [key]: key === "height" || key === "weight"
                      ? parseInt(e.target.value) || 0
                      : e.target.value,
                  }))}
                />
              )}
              <label className="flex items-center space-x-1 text-xs text-gray-500 shrink-0">
                <input
                  type="checkbox"
                  checked={isHidden(key)}
                  onChange={() => toggleHiddenField(key)}
                />
                <span>非公開</span>
              </label>
            </div>
          ))}
        </div>

        {/* 写真 */}
        <div className="mb-4 space-y-3">
          <h3 className="text-sm font-medium text-gray-700">プロフィール写真</h3>

          {/* Formal */}
          <div className="flex items-center space-x-3">
            <span className="w-32 text-sm text-gray-600 shrink-0">公式バストアップ <span className="text-red-500">*</span></span>
            {profile.portrait_formal_url && (
              <img src={profile.portrait_formal_url} className="w-16 h-16 object-cover rounded" alt="formal" />
            )}
            <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={formalRef} className="hidden"
              onChange={e => e.target.files?.[0] && handlePhotoUpload("formal", e.target.files[0])} />
            <button className="text-sm border rounded px-3 py-1" onClick={() => formalRef.current?.click()}>
              {profile.portrait_formal_url ? "変更" : "アップロード"}
            </button>
            <label className="flex items-center space-x-1 text-xs text-gray-500">
              <input type="checkbox" checked={isHidden("portrait_formal")} onChange={() => toggleHiddenField("portrait_formal")} />
              <span>非公開</span>
            </label>
          </div>

          {/* Casual */}
          <div className="flex items-center space-x-3">
            <span className="w-32 text-sm text-gray-600 shrink-0">カジュアルバストアップ <span className="text-red-500">*</span></span>
            {profile.portrait_casual_url && (
              <img src={profile.portrait_casual_url} className="w-16 h-16 object-cover rounded" alt="casual" />
            )}
            <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={casualRef} className="hidden"
              onChange={e => e.target.files?.[0] && handlePhotoUpload("casual", e.target.files[0])} />
            <button className="text-sm border rounded px-3 py-1" onClick={() => casualRef.current?.click()}>
              {profile.portrait_casual_url ? "変更" : "アップロード"}
            </button>
            <label className="flex items-center space-x-1 text-xs text-gray-500">
              <input type="checkbox" checked={isHidden("portrait_casual")} onChange={() => toggleHiddenField("portrait_casual")} />
              <span>非公開</span>
            </label>
          </div>

          {/* Additional */}
          <div className="flex items-start space-x-3">
            <span className="w-32 text-sm text-gray-600 shrink-0 pt-1">追加写真</span>
            <div className="flex-grow">
              <div className="flex flex-wrap gap-2 mb-2">
                {(profile.additional_photo_urls || []).map((url, i) => (
                  <img key={i} src={url} className="w-16 h-16 object-cover rounded" alt={`追加${i + 1}`} />
                ))}
              </div>
              <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" ref={additionalRef} className="hidden"
                onChange={e => e.target.files?.[0] && handlePhotoUpload("additional", e.target.files[0])} />
              <button className="text-sm border rounded px-3 py-1" onClick={() => additionalRef.current?.click()}>
                追加
              </button>
            </div>
            <label className="flex items-center space-x-1 text-xs text-gray-500 shrink-0 pt-1">
              <input type="checkbox" checked={isHidden("additional_photos")} onChange={() => toggleHiddenField("additional_photos")} />
              <span>非公開</span>
            </label>
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
