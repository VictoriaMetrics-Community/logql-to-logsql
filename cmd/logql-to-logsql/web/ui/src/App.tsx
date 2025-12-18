import { Main } from "@/pages/main";
import {Toaster} from "@/components/ui/sonner.tsx";

function App() {
  return (
    <>
      <Main />
      <Toaster richColors={true} position={"top-right"} closeButton={true} />
    </>
  );
}

export default App;
