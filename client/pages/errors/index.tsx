import Image from "next/image";
import { useRouter } from "next/router"

export default function Errors() {
  const router = useRouter();
  const code = router.query["code"] as string;
  return (
    <section className="h-screen w-screen flex flex-col items-center justify-center bg-gray-100">
      <h1 className="text-center font-bold text-xl py-2">エラー: {code}</h1>
      <div className="mx-8 mb-4">
        <ErrorDescriptionForCode code={code} />
      </div>
      <div className="w-2/3"><ErrorIcon /></div>
      <div>ゴメンね！</div>
    </section>
  )
}

function ErrorDescriptionForCode(props: { code: string }) {
  const {code} = props;
  if (!code) return <ErrorUndefined />;
  switch (code) {
  case "1001": return <ErrorMemberNotSyncedYet />;
  default: return <ErrorUnknown />;
  }
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

function ErrorIcon() {
  return <Image
    loader={({ src }) => src}
    alt={"エラ〜"}
    src={"https://pbs.twimg.com/profile_images/1157223157008179201/W7TsC5mH_400x400.jpg"}
    width={400}
    height={400}
  />;
}