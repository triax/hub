import { Fragment, useEffect, useState } from "react";
import Layout from "../../components/layout";

async function listMembers() {
  const endpoint = process.env.API_BASE_URL + "/api/1/members";
  const res = await fetch(endpoint, {
    cache: "no-cache",
  });
  return res.json();
}

export default function Members(props) {

  const [members, setMembers] = useState([]);
  useEffect(() => {
    setTimeout(() => { // FIXME: クソすぎる
      listMembers().then(mems => setMembers(mems));
    });
  }, []);

  return (
    <Layout {...props}>
      <div className="flex flex-col">
        <div className="-my-2 overflow-x-auto sm:-mx-6 lg:-mx-8">
          <div className="py-2 align-middle inline-block min-w-full sm:px-6 lg:px-8">
            <div className="shadow overflow-hidden border-b border-gray-200 rounded">
              <table className="min-w-full divide-y divide-gray-200">
                <MemberHead />
                <tbody className="bg-white divide-y divide-gray-200">
                  {members.map(m => <MemberRow key={m.slack.id} {...m} />)}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  )
}

function MemberHead() {
  return (
    <thead className="bg-gray-50">
      <tr>
        <th scope="col" className="font-medium text-gray-500 uppercase">#</th>
        <th
          scope="col"
          className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
        >
          Name
        </th>
        <th
          scope="col"
          className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
        >
          Position
        </th>
        <th
          scope="col"
          className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
        >
          Status
        </th>
        <th
          scope="col"
          className="relative px-6 py-3"
        >
          <span className="sr-only">Edit</span>
        </th>
      </tr>
    </thead>
  );
}

function MemberRow(member) {
  const { slack } = member;
  return (
    <tr key={slack.id}>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
        99
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="flex items-center">
          <div className="flex-shrink-0 h-10 w-10">
            <img
              className="h-10 w-10 rounded-full"
              src={slack.profile.image_512}
              alt={slack.profile.real_name}
            />
          </div>
          <div className="ml-4">
            <div className="text-sm font-medium text-gray-900">{slack.profile.real_name}</div>
            <div className="text-sm text-gray-500">{slack.id}</div>
          </div>
        </div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="text-sm text-gray-900">LB</div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <StatusBadges {...member} />
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
        <a
          href={`/members/${slack.id}`}
          className="text-indigo-600 hover:text-indigo-900">
          Edit
        </a>
      </td>

    </tr>
  );
}

function StatusBadges({slack}) {
  const badges = [];
  if (slack.deleted) {
    badges.push(
      <span key="is_deleted"
        className="px-2 inline-flex text-xs leading-5 font-semibold rounded-full bg-gray-200 text-gray-800">
          Deleted
      </span>
    );
  }
  return <Fragment>{badges}</Fragment>;
}