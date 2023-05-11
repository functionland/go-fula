Pod::Spec.new do |s|  
    s.name              = 'FulaMobile' # Name for your pod
    s.version           = '0.1.6'
    s.summary           = 'Go-fula for iOS'
    s.homepage          = 'https://github.com/functionland/go-fula'

    s.author            = { 'Functionland' => 'info@fx.land' }
    s.license = { :type => 'MIT', :file => 'LICENSE' }

    s.platform          = :ios
    # change the source location
    # s.source            = { :http => "https://github.com/functionland/go-fula/releases/download/v#{s.version}/cocoapods-bundle.zip" } 
    s.source            = { :git => "file://#{__dir__}" } 
    # s.source_files = "include/*.{h}"
    # s.module_map = "include/module.modulemap"
    s.ios.deployment_target = '11.0'
    # s.ios.vendored_libraries = 'libfula_iossimulator.a'
    # s.osx.vendored_libraries = 'libfula_darwin.a'
    s.vendored_frameworks = 'Fula.xcframework'
    # s.static_framework = true
end 