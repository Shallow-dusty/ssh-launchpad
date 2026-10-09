import { wireDefaultProfile, type WireProfile } from "../../build/contracts/wire-types";
import type { Profile } from "./types";

export const defaultProfile: Profile = {
  ...wireDefaultProfile,
  name: "recommended",
  labels: { ...wireDefaultProfile.labels, experience: "guided" }
};

export function normalizeProfile(profile: Partial<WireProfile>): Profile {
  const source = profile;
  const defaults = structuredClone(defaultProfile);
  return {
    ...defaults,
    ...source,
    target: { ...defaults.target, ...source.target },
    ssh: {
      ...defaults.ssh,
      ...source.ssh,
      publicKeys: Array.isArray(source.ssh?.publicKeys) ? [...source.ssh.publicKeys] : []
    },
    transport: { ...defaults.transport, ...source.transport },
    exposure: {
      ...defaults.exposure,
      ...source.exposure,
      customCidrs: Array.isArray(source.exposure?.customCidrs) ? [...source.exposure.customCidrs] : []
    },
    download: { ...defaults.download, ...source.download },
    safety: { ...defaults.safety, ...source.safety },
    advanced: { ...defaults.advanced, ...source.advanced },
    labels: { ...defaults.labels, ...(source.labels ?? {}) }
  };
}
