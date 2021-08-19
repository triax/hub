import { CSSProperties } from "react";

const styles: {[name:string]: CSSProperties} = {
    navbar: {
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
    },
    navitem: {
        flex: 1,
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
    },
};

export default function Layout({children, myself}) {
    // https://bulma.io/documentation/layout/container/
    console.log(myself);
    return (
        <div className="container">
            <nav className="navbar" style={styles.navbar}>
                <div style={styles.navitem}>
                    <a href="/">
                        <img
                            src={myself["https://slack.com/team_image_44"]}
                            alt={myself["https://slack.com/team_name"]}
                        />
                    </a>
                </div>
                <div style={styles.navitem}>
                    <span>
                        {myself["given_name"]}
                    </span>
                    <a href="/">
                        <img
                            src={myself["https://slack.com/user_image_48"]}
                            alt={myself["given_name"]}
                        />
                    </a>
                </div>
            </nav>
            <section>{children}</section>
        </div>
    );
}
