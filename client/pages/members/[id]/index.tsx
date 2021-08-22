import Layout from "../../../components/layout";
import { useRouter } from 'next/router';
import { useEffect, useState } from "react";

export default function Member(props) {
  const id = useRouter().query.id;
  const [member, setMember] = useState({});
  useEffect(() => {
    if (!id) return;
    // TODO: Repositoryつくる
    const endpoint = process.env.API_BASE_URL + `/api/1/members/${id}`;
    fetch(endpoint).then(res => res.json()).then(res => setMember(res));
  }, [id]);

  return (
    <Layout {...props}>
      <div>
        <h1>{id}</h1>
      </div>
    </Layout>
  )
}