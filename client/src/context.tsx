import { createContext, useContext, useEffect, useMemo, useState } from "react";
import MemberRepo from "../repository/MemberRepo";
import Member from "../models/Member";
import { useRouter } from "@tanstack/react-router";

interface AppContextValue {
  myself: Member;
  isLoading: boolean;
  startLoading: () => void;
  stopLoading: () => void;
}

const AppContext = createContext<AppContextValue>({
  myself: Member.placeholder(),
  isLoading: false,
  startLoading: () => {},
  stopLoading: () => {},
});

export function AppProvider({ children }: { children: React.ReactNode }) {
  const repo = useMemo(() => new MemberRepo(), []);
  const [isLoading, setIsLoading] = useState(false);
  const [myself, setMyself] = useState<Member>(Member.placeholder());
  const router = useRouter();

  useEffect(() => {
    const path = router.state.location.pathname;
    if (path === "/login" || path === "/errors") return;
    repo.myself().then(setMyself);
  }, [router.state.location.pathname, repo]);

  return (
    <AppContext.Provider value={{
      myself,
      isLoading,
      startLoading: () => setIsLoading(true),
      stopLoading: () => setIsLoading(false),
    }}>
      {children}
    </AppContext.Provider>
  );
}

export function useAppContext() {
  return useContext(AppContext);
}
