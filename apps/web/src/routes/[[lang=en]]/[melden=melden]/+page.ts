import { redirect } from "@sveltejs/kit";

export const load = ({ params, url }) => {
  const en = params.lang === "en";
  // /en/report und /melden sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.melden === "report")) redirect(308, (en ? "/en/report" : "/melden") + url.search);
  return { projekt: url.searchParams.get("project") ?? "", bezug: url.searchParams.get("location") ?? "" };
};
