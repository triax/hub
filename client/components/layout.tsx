
export default function Layout({children, myself}) {
    // https://bulma.io/documentation/layout/container/
    return (
        <div className="container">
            <nav className="navbar"></nav>
            <section>{children}</section>
        </div>
    );
}
