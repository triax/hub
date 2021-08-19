// これは build time で評価されるので、あまり使えない
// export async function getStaticProps() {
//     const res = await fetch("http://localhost:8080/api/1/users/current");
//     console.log(await res.json());
//     return {
//         props: {
//             message: "yay",
//         },
//     };
// }

export default function Top(props) {
    console.log("HMR?", props);
    return (
        <a href="/hoge">Move to Hoge</a>
    );
}
