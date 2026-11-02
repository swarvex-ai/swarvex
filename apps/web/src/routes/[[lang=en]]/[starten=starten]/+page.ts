import { redirect } from "@sveltejs/kit";

export const load = ({ params }) => {
  const en = params.lang === "en";
  // /en/start und /starten sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.starten === "start")) redirect(308, en ? "/en/start" : "/starten");
};
