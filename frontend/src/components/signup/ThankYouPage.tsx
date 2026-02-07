import { useEffect } from "react";

interface Props {
  data: Record<string, unknown>;
}

export default function ThankYouPage({ data }: Props) {
  const heading = (data.heading as string) || "Thank You!";
  const body = (data.body as string) || "Your membership has been successfully created.";
  const imageUrl = data.image_url as string | undefined;
  const redirectUrl = data.redirect_url as string | undefined;

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

      <h1 className="text-3xl font-bold text-gray-900 sm:text-4xl">{heading}</h1>
      <p className="mt-4 text-lg text-gray-600 max-w-lg mx-auto whitespace-pre-line">
        {body}
      </p>

      {redirectUrl && (
        <p className="mt-6 text-sm text-gray-400">
          You will be redirected in a few seconds...
        </p>
      )}
    </div>
  );
}
