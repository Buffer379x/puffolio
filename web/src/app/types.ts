export type FieldType =
  | 'text'
  | 'longtext'
  | 'rating'
  | 'image'
  | 'tags'
  | 'color'
  | 'date'
  | 'number'
  | 'select'
  | 'boolean';
export type Scalar = string | number | boolean | string[] | null;
export type Value = { type: string; value: Scalar };
export type Values = Record<string, Value | null>;
export interface Field {
  id: string;
  label: string;
  type: FieldType;
  position: number;
  version: number;
  required: boolean;
  requiredFrom?: string;
  archived: boolean;
  visibility: { form: boolean; detail: boolean; list: boolean; grid: boolean };
  config: {
    scale?: number;
    unit?: string;
    min?: string;
    max?: string;
    stars?: number;
    options?: { id: string; label: string; archived?: boolean; paletteId?: string }[];
    defaultValue?: Scalar;
    defaultToday?: boolean;
    searchable?: boolean;
    filterable?: boolean;
  };
}
export interface Tag {
  id: string;
  fieldId: string;
  name: string;
  paletteId: string;
  version: number;
}
export interface User {
  id: string;
  username: string;
  displayName: string;
  role: 'admin' | 'user' | 'visitor';
  version: number;
  disabled?: boolean;
}
export interface Me {
  user: User;
  collectionId: string;
  csrfToken: string;
}
export interface Liquid {
  id: string;
  name: string;
  favorite: boolean;
  version: number;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
  values: Values;
  /** Derived by the server; absent when no bar applies. */
  shelfLife?: { percent: number; expiresOn: string; expired: boolean };
}
export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}
export interface Preferences {
  version: number;
  theme: 'light' | 'dark' | 'system';
  timezone: string;
  view: 'grid' | 'list';
  images: boolean;
  listFields: string[];
  gridFields: string[];
  roles: Partial<
    Record<'image' | 'rating' | 'color' | 'purchase' | 'shelfOverride' | 'status', string>
  >;
  /** Default storage time from the purchase date; a liquid may override it. */
  shelfLifeMonths: number;
  /** Option of the status field that means used up: no bar, never expired. */
  consumedOption?: string;
}
export interface Filter {
  op?: 'and' | 'or';
  conditions?: Filter[];
  fieldId?: string;
  operator?: string;
  value?: Scalar;
  value2?: Scalar;
}
export interface SavedView {
  id: string;
  name: string;
  favorite: boolean;
  search: string;
  filters?: Filter;
  sort: string;
  direction: string;
  version: number;
}
export interface Crop {
  x: number;
  y: number;
  width: number;
  height: number;
}
export interface Media {
  id: string;
  width: number;
  height: number;
  crop?: Crop;
  url: string;
  masterUrl: string;
  thumbnailUrl: string;
}
export type WidgetType =
  | 'count'
  | 'sum'
  | 'favorites'
  | 'average'
  | 'recent'
  | 'low'
  | 'tags'
  | 'dates'
  | 'expired'
  | 'expiring'
  | 'donut'
  | 'ratings'
  | 'shelfStatus'
  | 'timeline'
  | 'top';
export interface Widget {
  id: string;
  type: WidgetType;
  title: string;
  size: 'small' | 'medium' | 'wide';
  fieldId?: string;
  limit?: 3 | 5 | 10;
  threshold?: string;
  inclusive?: boolean;
  from?: string;
  to?: string;
  window?: 'past' | 'upcoming';
  filters?: Filter;
}
export interface Layout {
  version: number;
  widgets: Widget[];
}
export interface Segment {
  id: string;
  label: string;
  count: number;
  paletteId?: string;
}
export interface WidgetResult {
  id: string;
  value?: string;
  count?: number;
  items?: Liquid[];
  tags?: { id: string; name: string; paletteId: string; count: number }[];
  segments?: Segment[];
  error?: string;
}
