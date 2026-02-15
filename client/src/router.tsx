import { createRouter, createRoute, createRootRoute, Outlet } from '@tanstack/react-router';
import { AppProvider } from './context';

import Top from './pages/index';
import Login from './pages/login';
import Members from './pages/members';
import MemberView from './pages/members.$id';
import Events from './pages/events';
import EventView from './pages/events.$id';
import EquipsList from './pages/equips';
import EquipCreate from './pages/equips.create';
import EquipReport from './pages/equips.report';
import EquipView from './pages/equips.$id';
import EquipEdit from './pages/equips.$id.edit';
import Uniforms from './pages/uniforms';
import Errors from './pages/errors';

const rootRoute = createRootRoute({
  component: () => (
    <AppProvider>
      <Outlet />
    </AppProvider>
  ),
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: Top,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  component: Login,
});

const membersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/members',
  component: Members,
});

const memberRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/members/$id',
  component: MemberView,
});

const eventsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/events',
  component: Events,
});

const eventRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/events/$id',
  component: EventView,
});

const equipsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/equips',
  component: EquipsList,
});

const equipCreateRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/equips/create',
  component: EquipCreate,
});

const equipReportRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/equips/report',
  component: EquipReport,
});

const equipRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/equips/$id',
  component: EquipView,
});

const equipEditRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/equips/$id/edit',
  component: EquipEdit,
});

const uniformsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/uniforms',
  component: Uniforms,
});

const errorsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/errors',
  component: Errors,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  loginRoute,
  membersRoute,
  memberRoute,
  eventsRoute,
  eventRoute,
  equipsRoute,
  equipCreateRoute,
  equipReportRoute,
  equipRoute,
  equipEditRoute,
  uniformsRoute,
  errorsRoute,
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
