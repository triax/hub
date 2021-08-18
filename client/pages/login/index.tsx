import Link from "next/link";

export default function Login() {
    return (
        <div>
            <Link href="/?u=foo&p=baa">Login and jump to top</Link>
        </div>
    )
}