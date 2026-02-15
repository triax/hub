import { useSearch } from "@tanstack/react-router";

export default function Errors() {
  const search: Record<string, string> = useSearch({ strict: false });
  const code: number = parseInt(search.code as string);
  const error: string = search.error as string;
  return (
    <section className="h-screen w-screen flex flex-col items-center justify-center bg-gray-100">
      <h1 className="text-center font-bold text-xl py-2">エラー: {code}</h1>
      {error ? <pre className="text-gray-500 text-xs bg-slate-200 whitespace-pre-wrap p-2 rounded-sm">{error}</pre> : null}
      <div className="mx-8 mb-4">
        <ErrorDescriptionForCode code={code} />
      </div>
      <div className="w-2/3 flex justify-center"><ErrorIcon /></div>
      <div>ゴメンね！</div>
    </section>
  )
}

const ErrorMap = {
  1001: ErrorMemberNotSyncedYet,
  4001: ErrorDatastoreClientInit,
  4002: ErrorDatastoreObjectMap,
}

function ErrorDescriptionForCode(props: { code: number }) {
  const {code} = props;
  if (!code) return <ErrorUndefined />;
  const Elem = ErrorMap[code];
  if (!Elem) return <ErrorUnknown />;
  return <Elem />;
}

function ErrorMemberNotSyncedYet() {
  return <div className="text-center">Slackによる認証は成功しましたが、Slackメンバー情報がまだHubへ反映されていないようです. 今日Slackに追加された場合に発生し、明日には解決するはずです. それでも解決しない場合は、Slackの #tech チャンネルにお問い合わせください.</div>;
}

function ErrorUndefined() {
  return <div>エラーコードが指定されていません. #tech チャンネルにて管理者にお問い合わせください.</div>;
}

function ErrorUnknown() {
  return <div>不明なエラーコードです. この画面のスクリーンショットを撮って、#tech チャンネルにて管理者にお問い合わせください.</div>;
}

function ErrorDatastoreClientInit() {
  return <__ErrorSystemError__ />;
}

function ErrorDatastoreObjectMap() {
  return <__ErrorSystemError__ />;
}

function __ErrorSystemError__() {
  return <div>システムエラーです. この画面のスクリーンショットを撮って、 #tech チャンネルにて管理者にお問い合わせください.</div>
}

function ErrorIcon() {
  return <img
    alt={"エラ〜"}
    src={"https://pbs.twimg.com/profile_images/1157223157008179201/W7TsC5mH_400x400.jpg"}
    width={400}
    height={400}
  />;
}
