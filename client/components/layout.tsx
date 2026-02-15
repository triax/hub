import React, { Fragment, useEffect } from "react";
import { Disclosure, Menu, Transition } from "@headlessui/react";
import { BellIcon, MenuIcon, RefreshIcon, XIcon } from "@heroicons/react/outline";
import { useLocation } from "@tanstack/react-router";
import { useAppContext } from "../src/context";

const defaultTeamIcon = "https://s3-us-west-2.amazonaws.com/slack-files2/avatars/2022-02-11/3102891044193_8ede5969380a68bc44bf_132.png";
const defaultTeamName = "Triax";

const HELP_PAGE_URL = import.meta.env.VITE_HELP_PAGE_URL || "https://sites.google.com/view/how-to-use-triax-hub";

const navigation = [
  { label: 'Schedule', link: '/' },
  { label: 'Equips', link: '/equips' },
  { label: 'Uniforms', link: '/uniforms' },
  { label: 'Team', link: '/members' },
];

if (HELP_PAGE_URL) {
  navigation.push({ label: 'Help', link: HELP_PAGE_URL });
}

function classnames(...classes) {
  return classes.filter(Boolean).join(' ');
}

function Loading({isLoading}) {
  if (!isLoading) return <></>;
  return (
    <div className="fixed w-full h-full bg-black bg-opacity-60 flex justify-center items-center space-x-2">
      <RefreshIcon color="white" className="w-10 h-10 animate-spin" />
      <span className="text-white text-xl">Loading...</span>
    </div>
  )
}

export interface LayoutProps {
  children?: React.ReactNode,
}

export default function Layout({ children }: LayoutProps) {
  const { myself, isLoading } = useAppContext();
  const { pathname } = useLocation();
  const teamIcon: string = myself.team?.icon?.image_132 || defaultTeamIcon;
  const teamName: string = myself.team?.name || defaultTeamName;
  const myIcon: string = myself.slack.profile.image_512;
  const title = `${import.meta.env.MODE === "production" ? "" : "[DEV] "}${teamName} Team Hub`;

  useEffect(() => {
    document.title = title;
    // Update favicon
    let link: HTMLLinkElement = document.querySelector("link[rel='shortcut icon']");
    if (!link) {
      link = document.createElement('link');
      link.rel = 'shortcut icon';
      document.head.appendChild(link);
    }
    link.href = teamIcon;
    let apple: HTMLLinkElement = document.querySelector("link[rel='apple-touch-icon']");
    if (!apple) {
      apple = document.createElement('link');
      apple.rel = 'apple-touch-icon';
      document.head.appendChild(apple);
    }
    apple.href = teamIcon;
  }, [title, teamIcon]);

  return (
    <div>
      <Loading isLoading={isLoading} />
      <Disclosure as="nav" className="bg-gray-800">
        {({open}) => (
          <>
            <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
              <div className="flex items-center justify-between h-16">

                {/* LEFT PART */}
                <div className="flex items-center">

                  {/* Always show branc-logo */}
                  <div
                    className="flex-shrink-0 flex items-center"
                    onClick={() => location.href = "/"}
                  >
                    <img className="h-8 w-8" src={teamIcon} alt="Triax" />
                    <span className="md:hidden ml-2 text-gray-100">Team HUB</span>
                  </div>

                  {/* Items HIDDEN in small */}
                  <div className="hidden md:block">
                    <div className="ml-10 flex items-baseline space-x-4">
                      {navigation.map(item => item.link == pathname ? (
                        <Fragment key={item.label}>
                          <a
                            href={item.link}
                            className="bg-gray-900 text-white px-3 py-2 rounded-md text-sm font-medium"
                          >{item.label}</a>
                        </Fragment>
                      ) : (
                        <a
                          key={item.label}
                          href={item.link}
                          className="text-gray-300 hover:bg-gray-700 hover:text-white px-3 py-2 rounded-md text-sm font-medium"
                        >{item.label}</a>
                      ))}
                    </div>
                  </div>

                </div>


                {/* RIGHT PART */}
                <div className="hidden md:block">
                  <div className="ml-4 flex items-center md:ml-6">
                    <button type="button"
                      className="bg-gray-800 p-1 rounded-full text-gray-400 hover:text-white focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-white"
                    >
                      <span className="sr-only">View notifications</span>
                      <BellIcon className="h-6 w-6" aria-hidden="true" />
                    </button>

                    {/* Profile dropdown */}
                    <Menu as="div" className="ml-3 relative">
                      <div>
                        <Menu.Button
                          className="max-w-xs bg-gray-800 rounded-full flex items-center text-sm focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-white"
                        >
                          <span className="sr-only">Open user menu</span>
                          {myIcon ? <img
                            className="h-8 w-8 rounded-full"
                            src={myIcon}
                            alt={myself.slack.profile.real_name}
                          /> : null}
                        </Menu.Button>
                      </div>
                      <Transition
                        as={Fragment}
                        enter="transition ease-out duration-100"
                        enterFrom="transform opacity-0 scale-95"
                        enterTo="transform opacity-100 scale-100"
                        leave="transition ease-in duration-75"
                        leaveFrom="transform opacity-100 scale-100"
                        leaveTo="transform opacity-0 scale-95"
                      >
                        <Menu.Items
                          className="origin-top-right absolute right-0 mt-2 w-48 rounded-md shadow-lg py-1 bg-white ring-1 ring-black ring-opacity-5 focus:outline-none"
                        >
                          <Menu.Item key={"Your Profile"}>
                            {({active}) => (
                              <span onClick={() => location.href = `/members/${myself.slack.id}`}
                                className={classnames(
                                  active ? 'bg-gray-100' : '',
                                  'block px-4 py-2 text-sm text-gray-700',
                                )}
                              >Your Profile</span>
                            )}
                          </Menu.Item>
                          <Menu.Item key={"Sign out"}>
                            {({active}) => (
                              <form method="POST" action="/logout" className={classnames(
                                active ? 'bg-gray-100' : '',
                                'block px-4 py-2 text-sm text-gray-700',
                              )}>
                                <input type="submit" value="Sign Out" className="bg-transparent" />
                              </form>
                            )}
                          </Menu.Item>
                        </Menu.Items>
                      </Transition>
                    </Menu>
                  </div>
                </div>

                <div className="-mr-2 flex md:hidden">
                  {/* Mobile menu button */}
                  <Disclosure.Button
                    className="bg-gray-800 inline-flex items-center justify-center p-2 rounded-md text-gray-400 hover:text-white hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-white"
                  >
                    <span className="sr-only">Open main menu</span>
                    {open ? (
                      <XIcon className="block h-6 w-6" aria-hidden="true" />
                    ) : (
                      <MenuIcon className="block h-6 w-6" aria-hidden="true" />
                    )}
                  </Disclosure.Button>
                </div>
              </div>
            </div>

            <Disclosure.Panel className="md:hidden">
              <div className="px-2 pt-2 pb-3 space-y-1 sm:px-3">
                {navigation.map((item, i) => i === 0 ? (
                  <Fragment key={item.label}>
                    <a href={item.link} className="bg-gray-900 text-white block px-3 py-2 rounded-md text-base font-medium">{item.label}</a>
                  </Fragment>
                ) : (
                  <a key={item.label} href={item.link} className="text-gray-300 hover:bg-gray-700 hover:text-white block px-3 py-2 rounded-md text-base font-medium">{item.label}</a>
                ))}
              </div>
              <div className="pt-4 pb-3 border-t border-gray-700">
                <div className="flex items-center px-5">
                  <div className="flex-shrink-0">
                    {myIcon ? <img
                      className="h-10 w-10 rounded-full"
                      src={myIcon}
                      alt={myself.slack.profile.real_name}
                    /> : null}
                  </div>
                  <div className="ml-3">
                    <div className="text-base font-medium leading-none text-white">{myself.slack.profile.real_name}</div>
                    <div className="text-sm font-medium leading-none text-gray-400">tom@example.com</div>
                  </div>

                  <button
                    type="button"
                    className="ml-auto bg-gray-800 flex-shrink-0 p-1 rounded-full text-gray-400 hover:text-white focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-800 focus:ring-white"
                  >
                    <span className="sr-only">View Notifications</span>
                    <BellIcon className="h-6 w-6" aria-hidden="true" />
                  </button>
                </div>
                <div className="mt-3 px-2 space-y-1">
                  <form
                    method="POST" action="/logout"
                    className="block px-3 py-2 rounded-md text-base font-medium text-gray-400 hover:text-white hover:bg-gray-700"
                  >
                    <input type="submit" value="Sign Out" className="bg-transparent" />
                  </form>
                </div>
              </div>
            </Disclosure.Panel>

          </>
        )}
      </Disclosure>

      <main>
        <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          {children}
        </div>
      </main>

    </div>
  );
}
