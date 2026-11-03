// /mitstreiter/<login> (Deutsch) und /en/contributors/<login> (Englisch): öffentliches Beitragsprofil.
export const match = (p: string) => p === "mitstreiter" || p === "contributors";
