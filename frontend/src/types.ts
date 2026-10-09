// Wire shapes and Wails signatures are generated from Go before typechecking.
// Only normalized UI state and the user-facing stage subset are defined here.
import type {
  WireAdvancedProfile, WireDesktopRequest, WireDownloadProfile,
  WireExposureProfile, WireProfile, WireSSHProfile, WireStage
} from "../../build/contracts/wire-types";
import type { DesktopBridge, WireEvent } from "../../build/contracts/wire-types";

export type {
  DesktopBridge,
  WireAction as PlanAction,
  WireElevatedJob as ElevatedJob,
  WireFailureReason as FailureReason,
  WirePersonalCard as PersonalCard,
  WirePlan as Plan,
  WirePublicKeyInfo as PublicKeyInfo,
  WireReport as Report,
  WireRisk as Risk,
  WireSnapshot as Snapshot,
  WireUpdateInfo as UpdateInfo
} from "../../build/contracts/wire-types";

export type Stage = Exclude<WireStage, "rollback">;

// Go JSON may omit empty strings and encode nil slices/maps as null. Forms
// operate on normalized values, so these refinements make that boundary explicit.
export type Profile = Omit<WireProfile, "ssh" | "exposure" | "download" | "advanced" | "labels"> & {
  ssh: Omit<WireSSHProfile, "publicKeys"> & {
    publicKeys: NonNullable<WireSSHProfile["publicKeys"]>;
  };
  exposure: Omit<WireExposureProfile, "customCidrs"> & {
    customCidrs: NonNullable<WireExposureProfile["customCidrs"]>;
  };
  download: Required<WireDownloadProfile>;
  advanced: Required<WireAdvancedProfile>;
  labels: NonNullable<WireProfile["labels"]>;
};

export type DesktopRequest = Omit<WireDesktopRequest, "profile" | "stage" | "planDigest"> & {
  profile: Profile;
  stage: Stage;
  planDigest: string;
};

// Compile-time checks keep UI refinements assignable to the Go wire contract.
type Assert<T extends true> = T;
type ProfileContract = Assert<Profile extends WireProfile ? true : false>;
type RequestContract = Assert<DesktopRequest extends WireDesktopRequest ? true : false>;

declare global {
  interface Window {
    go?: {
      main?: {
        App?: DesktopBridge;
      };
    };
    runtime?: {
      EventsOn(name: string, callback: (event: WireEvent) => void): void;
    };
  }
}
