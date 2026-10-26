import { redirect } from "@sveltejs/kit";

export const load = ({ params }) => {
  const en = params.lang === "en";
  // /en/account und /konto sind die richtigen Adressen, die Mischformen leiten um
  if (en !== (params.konto === "account")) redirect(308, en ? "/en/account" : "/konto");
};
