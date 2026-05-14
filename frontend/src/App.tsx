import { RouterProvider } from "@tanstack/react-router";
import { router } from "./router";
import { useSessionTimeout } from "@/hooks/useSessionTimeout";

function App() {
  useSessionTimeout();
  return <RouterProvider router={router} />;
}

export default App;
