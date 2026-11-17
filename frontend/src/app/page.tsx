import { redirect } from "next/navigation";
/** Routes the entry URL to the dashboard, whose client guard handles session restoration. */
export default function Home() { redirect("/dashboard"); }
