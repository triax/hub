export interface ApplicationStep {
  key: string;
  label: string;
  done: boolean;
}

export default interface Application {
  id: string;
  type: string;
  email: string;
  name: string;
  fields: Record<string, string>;
  consent_agreed_at: string;
  steps: ApplicationStep[];
  done: boolean;
  created_at: string;
}
