import { createRouter, createRoute, createRootRoute, Outlet } from '@tanstack/react-router';
import { AppProvider } from './context';
import ErrorFallback from './ErrorFallback';
import { reportClientError } from '../repository/observability';

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
import TapingRequestPage from './pages/taping.request';
import TapingMasterPage from './pages/taping.master';
import TapingOverviewPage from './pages/taping';
import EventTapingPage from './pages/events.$id.taping';
import OnboardingForm from './pages/applications.onboarding';
import ApplicationsPage from './pages/applications';

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

const tapingRequestRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/taping/request',
  component: TapingRequestPage,
});

const tapingMasterRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/taping/master',
  component: TapingMasterPage,
});

const tapingOverviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/taping',
  component: TapingOverviewPage,
});

const eventTapingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/events/$id/taping',
  component: EventTapingPage,
});

const applicationsOnboardingRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/applications/onboarding',
  component: OnboardingForm,
});

const applicationsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/applications',
  component: ApplicationsPage,
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
  tapingRequestRoute,
  tapingMasterRoute,
  tapingOverviewRoute,
  eventTapingRoute,
  applicationsOnboardingRoute,
  applicationsRoute,
]);

export const router = createRouter({
  routeTree,
  // route 描画中の例外を全 route で捕捉してフォールバック表示する
  defaultErrorComponent: ErrorFallback,
  // 例外の報告はここで行う。defaultErrorComponent の props には componentStack が
  // 渡らない（空文字固定）ため、React のコンポーネントスタックを含む errorInfo を
  // 受け取れる defaultOnCatch でサーバへ報告する（描画クラッシュの犯人特定に有効）。
  defaultOnCatch: (error, errorInfo) => {
    reportClientError({
      message: error?.message || String(error),
      stack: error?.stack,
      componentStack: errorInfo?.componentStack,
    });
  },
});

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
