import { useEffect } from "react";

interface Props {
  data: Record<string, unknown>;
}

export default function ThankYouPage({ data }: Props) {
  const heading = (data.heading as string) || "Thank you";
  const body = (data.body as string) || "Your membership has been successfully created.";
  const imageUrl = data.image_url as string | undefined;
  const redirectUrl = data.redirect_url as string | undefined;
  const gcError = data.gocardless_error as string | undefined;

  // Auto-redirect after delay if redirect URL is set
  useEffect(() => {
    if (redirectUrl) {
      const timer = setTimeout(() => {
        window.location.href = redirectUrl;
      }, 5000);
      return () => clearTimeout(timer);
    }
  }, [redirectUrl]);

  return (
    <div className="text-center py-12 sm:py-20">
      {imageUrl && (
        <img src={imageUrl} alt="" className="mx-auto mb-8 max-w-sm rounded-xl" />
      )}

      <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-full bg-green-100 mb-6">
        <svg className="h-10 w-10 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      </div>

      <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">{heading}</h1>
      <p className="mt-4 text-lg text-gray-600 max-w-lg mx-auto whitespace-pre-line">
        {body}
      </p>

      {gcError && (
        <div className="mt-6 mx-auto max-w-lg rounded-xl bg-amber-50 border border-amber-200 px-5 py-4 flex items-start gap-3">
          <svg className="h-5 w-5 text-amber-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126z" /></svg>
          <p className="text-sm text-amber-800">{gcError}</p>
        </div>
      )}

      {redirectUrl && (
        <p className="mt-6 text-sm text-gray-400">
          Redirecting shortly...
        </p>
      )}
    </div>
  );
}
