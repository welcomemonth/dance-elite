import React from 'react';

/** 表单字段：标签通过 htmlFor 关联控件，控件由调用方以 children 传入。 */
const Field: React.FC<{
  id: string;
  label: string;
  children: React.ReactNode;
}> = ({ id, label, children }) => (
  <div className="lp-field">
    <label className="lp-label" htmlFor={id}>
      {label}
    </label>
    {children}
  </div>
);

export default Field;
