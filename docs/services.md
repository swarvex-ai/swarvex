# Services: rights to generated results

Background research for the non-binding plan note in `contribution-statement.md`. **Responsibility for rights lies with the contributors, not with the platform** (decision neVs, 24 Oct 2026). This page creates no duty to check. Per service and plan: who owns the result, and may it go into an open-source project? Every row names its source and the date it was checked. **Not legal advice.**

## Basic rule: "non-commercial" never fits an open-source project

Even if a project sells nothing itself: MIT and CC BY allow **everyone** commercial use. A result that may only be used "privately/non-commercially" can therefore not be put under an open license. In practice: **the free plans of almost all services are out.** Exceptions are services that put free results under an open license themselves (Meshy: CC BY 4.0).

## Meshy

Source: Terms of Service, "Last Updated: September 19, 2026", checked on 24 Oct 2026 (https://www.meshy.ai/terms-of-use)

| Plan | Ownership of the result | License | For open-source projects |
|---|---|---|---|
| Free | **Meshy** owns the result (§3.2) | CC BY 4.0, attribution "Meshy" required | **yes**, with attribution in the asset manifest and the credits |
| Paid | the **customer** owns the result (§3.2) | freely licensable | **yes** |
| Community page | posted results under CC0 (§3.3) | public domain | yes |

Further points:

- **Service assets (§2.3/§3.1):** animations, rigs and precomputed textures from the Meshy library are only licensed *as far as they are part of the result and needed to use it*. Passing them on as standalone files under an open license is not covered. **Consequence:** library animations taken out on their own (e.g. retargeted clips) do not simply fall under the project license but need a special note. Alternatively, create them anew via text-to-motion as a result of your own. This is a question of interpretation that should be clarified by an expert.
- **AI labelling (§2.4):** metadata and watermarks that Meshy embeds in results must not be removed or changed. **Consequence:** pipelines (retarget, re-export) must keep them. The asset manifest marks AI origin anyway.
- **Training (§2.6):** results must not be used to train models that compete with Meshy. That binds the creator, not the project. It still belongs in the manifest as a note.
- **API (§2.5):** Meshy deletes API results after 3 days, so the runner must save them right away.
- **Training by Meshy (§2.9):** Meshy may use inputs and outputs of non-enterprise customers for training. Donors should know that.

## Higgsfield

Sources: help article "Who owns my generations and can I use them commercially?" and the summary of the terms change (effective 27 Aug 2026), checked on 24 Oct 2026. The full text of the terms (https://higgsfield.ai/terms-of-use-agreement) could not be read directly when retrieved.

| Plan | Ownership of the result | Commercial use | For open-source projects |
|---|---|---|---|
| all | the **creator** (§4.4); Higgsfield claims no ownership, rights remain after cancellation | not restricted according to terms and help, not tied to a plan | **yes**, legally |
| Free | as above | A third party (Krea blog) reports that the pricing table lists "Commercial use" as *not included* in the free plan. That contradicts the terms and was not checked by us | **unclear** |
| Free | – | results carry a **watermark**; without watermark only in paid plans | practically unsuitable for intros/trailers |

Further points:

- **Training (§5.1(iii), §5.2(iv)):** results must not be used to train AI models. That binds the creator. Note in the manifest.
- **Training by Higgsfield (§4.4):** Higgsfield uses content for training (except enterprise).
- **Third-party rights (§5.1(ii)(b), §5.3):** the creator is liable for third-party rights (trademarks, faces, copyright). This mainly concerns reference images.

## Suno (music)

Source: Terms of Service, "Last revised August 10, 2026", effective 3 Sep 2026, checked on 24 Oct 2026 (https://suno.com/terms)

| Plan | Ownership | For open-source projects |
|---|---|---|
| Free (Basic) | use only "lawful, personal and non-commercial" | **no** |
| Pro / Premier | Suno assigns its rights in the result to the user. But the result stays "subject to these Terms … including any applicable commercial use restrictions", and Suno does not guarantee that copyright arises at all | **unclear → expert** |

Further points:

- **Commercial use only after a permitted download** through Suno, with a monthly quota. Rights to it remain after cancellation or downgrade.
- **Remixes** can only be used non-commercially regardless of plan, so never for open source.
- **Watermarks and metadata** must not be removed. Training ban for results.
- Whether a Pro user may put a song under CC BY, which allows *third parties* any commercial use, is left open by the terms ("distribute, license … beyond the permitted Download use" is prohibited). Until clarified: **Suno not on the license list.**

## ElevenLabs (voices, sound effects, music)

Source: official help article "Can I publish the content I generate on the platform?", checked on 24 Oct 2026

| Plan | Commercial | For open-source projects |
|---|---|---|
| Free / not signed in | "does not include a commercial license", attribution "elevenlabs.io" required | **no** |
| Paid | commercial license, unlimited in time for content created during the subscription; **not** for beta services | **likely yes** (still read the service-specific terms for sound effects/music) |

## Checked without an official source (secondary sources only, not on the license list)

| Service | According to secondary sources | Status |
|---|---|---|
| **Udio** (music) | ownership with Udio or its licensors; default "personal, non-commercial" | official terms not readable when retrieved. **No for now** |
| **MiniMax / Hailuo** (video, music) | Free: watermark, non-commercial. Paid: commercial, ownership with the user. Open H3 weights: the license excludes the EU, UK, South Korea and the USA, also for results | hosted service, paid: read the official terms. **Local MiniMax with open weights: not allowed in the EU according to a secondary source**; check carefully |
| **Kling** (video) | Free: non-commercial, watermark. Paid: commercial. Broad license to Kling/Kuaishou to use the content | read the official terms |
| **Tripo** (3D) | contradictory: the blog says Free = CC BY 4.0, the help page says Free = Tripo keeps all rights (terms 5.2.1). Paid: rights with the user (5.2.2) | **free: no**, paid after checking the terms |

## Consequences for the platform

Since the decision of 24 Oct 2026 there is no enforced license list. There is only:

1. the contribution statement, with which the contributor confirms the rights
2. a non-binding plan note (Meshy, Higgsfield, ElevenLabs, paid plans in each case)
3. a report form and a takedown process

See `contribution-statement.md`.
