import Head from "next/head";

import { Fragment } from "react";
import { Disclosure, Menu, Transition } from "@headlessui/react";
import { BellIcon, MenuIcon, RefreshIcon, XIcon } from "@heroicons/react/outline";

const navigation = [
  { label: 'Dashboard', link: '/' },
  { label: 'Calendar', link: '/' },
  { label: 'Team', link: '/members' }
];
const profile = ['Your Profile', 'Settings', 'Sign out']

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

export default function Layout({children, myself, isLoading}) {
  const teamIcon: string = myself.openid["https://slack.com/team_image_44"];
  const touchIcon: string = myself.openid["https://slack.com/team_image_132"];
  const myIcon: string = myself.openid["picture"];
  return (
    <div id="root">
      <Head>
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{process.env.NODE_ENV == "production" ? "" : "[DEV] "}Triax Team Hub</title>
        <link rel="apple-touch-icon" href={touchIcon} />
        <link rel="shortcut icon" href={teamIcon} />
      </Head>
      <Loading isLoading={isLoading} />
      <Disclosure as="nav" className="bg-gray-800">
        {({open}) => (
          <>
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
              <div className="flex items-center justify-between h-16">

                {/* LEFT PART */}
                <div className="flex items-center">

                  {/* Always show branc-logo */}
                  <div className="flex-shrink-0 flex items-center">
                    <img className="h-8 w-8" src={teamIcon} alt="Triax" />
                    <span className="md:hidden ml-2 text-gray-100">Team HUB</span>
                  </div>

                  {/* Items HIDDEN in small */}
                  <div className="hidden md:block">
                    <div className="ml-10 flex items-baseline space-x-4">
                      {navigation.map((item, i) => i == 0 ? (
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
                          <img
                            className="h-8 w-8 rounded-full"
                            src={myIcon}
                            alt={myself.openid.given_name}
                          />
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
                          {profile.map((item) => (
                            <Menu.Item key={item}>
                              {({active}) => (
                                <a href="#"
                                  className={classnames(
                                    active ? 'bg-gray-100' : '',
                                    'block px-4 py-2 text-sm text-gray-700',
                                  )}
                                >{item}</a>
                              )}
                            </Menu.Item>
                          ))}
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
                    <img
                      className="h-10 w-10 rounded-full"
                      src={myIcon}
                      alt={myself.openid.given_name}
                    />
                  </div>
                  <div className="ml-3">
                    <div className="text-base font-medium leading-none text-white">{myself.openid.given_name} {myself.openid.family_name}</div>
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
                  {profile.map((item) => (
                    <a
                      key={item}
                      href="#"
                      className="block px-3 py-2 rounded-md text-base font-medium text-gray-400 hover:text-white hover:bg-gray-700"
                    >{item}</a>
                  ))}
                </div>
              </div>
            </Disclosure.Panel>

          </>
        )}
      </Disclosure>

      {/* <header className="bg-white shadow">
        <div className="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
          <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
        </div>
      </header> */}

      <main>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          {children}
        </div>
      </main>

    </div>
  );
}
